package rest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/forms"
	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/calls/callstore"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/nexus"
	"dynatron.me/x/stillbox/pkg/sinks"
	"dynatron.me/x/stillbox/pkg/stats"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"github.com/microcosm-cc/bluemonday"
)

const (
	fileNameDateFmt = "2006-01-02_150405"
)

var (
	ErrNoCall          = errors.New("no call specified")
	ErrMustBePlaintext = errors.New("content type must be text/plain")
	ErrMustBeJSON      = errors.New("content type must be application/json")
)

type callsAPI struct {
	nex         nexus.Nexus
	htmlSani    *bluemonday.Policy
	transcripts sinks.Transcriber
}

func newCallsAPI(nex nexus.Nexus, transcripts sinks.Transcriber) *callsAPI {
	return &callsAPI{
		nex:         nex,
		htmlSani:    bluemonday.StrictPolicy(),
		transcripts: transcripts,
	}
}

func (ca *callsAPI) Subrouter() http.Handler {
	r := chi.NewMux()

	r.Get(`/{call:[a-f0-9-]+}`, ca.getAudioRoute)
	r.Get(`/{call:[a-f0-9-]+}/info`, ca.getCallInfoRoute)
	r.Get(`/{call:[a-f0-9-]+}/{download:download}`, ca.getAudioRoute)
	r.Post(`/{call:[a-f0-9-]+}/transcript`, ca.transcriptRoute)
	r.Post(`/transcribe`, ca.dispatchTranscriptRoute)
	r.Post(`/`, ca.listCalls)
	r.Get(`/stats/{interval}`, ca.getCallStats)

	return r
}

type getAudioParams struct {
	CallID   *uuid.UUID `param:"call"`
	Download *string    `param:"download"`
}

func (ca *callsAPI) getAudioRoute(w http.ResponseWriter, r *http.Request) {
	p := getAudioParams{}

	err := decodeParams(&p, r)
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	ca.getAudio(p, w, r)
}

func (ca *callsAPI) dispatchTranscriptRoute(w http.ResponseWriter, r *http.Request) {
	p := &struct {
		Calls jsontypes.UUIDs `json:"calls"`
	}{}

	err := forms.Unmarshal(r, &p)
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	ctx := r.Context()
	callst := callstore.FromCtx(ctx)
	calls, err := callst.CompleteCalls(ctx, p.Calls)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	var errs multierror.Error

	for _, c := range calls {
		err := ca.transcripts.UnfilteredCall(ctx, c)
		if err != nil {
			errs.Errors = append(errs.Errors, fmt.Errorf("%s: %w", c.ID.String(), err))
		}
	}

	if errs.Errors != nil {
		http.Error(w, errs.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// entityReplacer turns quote entities back into quotes; there does not seem to be a way to make
// bluemonday pass them through.
var entityReplacer = strings.NewReplacer("&#39;", "'", "&#34;", `"`)

func (ca *callsAPI) transcriptRoute(w http.ResponseWriter, r *http.Request) {
	p := struct {
		CallID uuid.UUID `param:"call"`
	}{}

	err := decodeParams(&p, r)
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	contentType := strings.Split(r.Header.Get("Content-Type"), ";")[0]
	if contentType != "application/json" {
		wErr(w, r, badRequestErrText(ErrMustBeJSON))
		return
	}

	ctx := r.Context()

	txc := struct {
		Text      string `json:"text"`
		ElapsedMS *int   `json:"elapsedMS"`
	}{}

	err = json.NewDecoder(r.Body).Decode(&txc)
	if err != nil {
		wErr(w, r, badRequestErrText(err))
		return
	}

	sani := ca.htmlSani.Sanitize(txc.Text)

	var txv *string
	if len(sani) > 0 {
		xsc := strings.Trim(entityReplacer.Replace(sani), " \t")
		if xsc != "" {
			txv = &xsc
		}
	}

	tsc, err := callstore.FromCtx(ctx).UpdateTranscription(ctx, p.CallID, txv)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	if txc.ElapsedMS != nil {
		ca.transcripts.TranscribeDuration(time.Duration(*txc.ElapsedMS) * time.Millisecond)
	}

	if ctx.Err() == nil {
		ca.nex.Broadcast(tsc)
	}

	w.WriteHeader(http.StatusNoContent)
}

func (ca *callsAPI) getCallStats(w http.ResponseWriter, r *http.Request) {
	p := struct {
		Interval calls.StatsInterval `param:"interval"`
	}{}

	err := decodeParams(&p, r)
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	ctx := r.Context()
	sts := stats.FromCtx(ctx)

	st, err := sts.GetCallStats(ctx, p.Interval)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	respond(w, r, st)
}

func (ca *callsAPI) getAudio(p getAudioParams, w http.ResponseWriter, r *http.Request) {
	if p.CallID == nil {
		wErr(w, r, badRequest(ErrNoCall))
		return
	}

	ctx := r.Context()
	calls := callstore.FromCtx(ctx)

	caOpts := make([]callstore.CallAudioOption, 0, 2)

	if p.Download != nil {
		caOpts = append(caOpts, callstore.WithDownloadDisposition(true))
	}
	call, err := calls.CallAudio(ctx, *p.CallID, append(caOpts, callstore.WithResponseWriter(w))...)
	switch err {
	case io.EOF: // sendfile(2)/splice(2) (TCPConn.ReadFrom()) was used by the fs backend
		return
	case nil:
		// continue
	default: // error
		wErr(w, r, autoError(err))
		return
	}

	if call.AudioBlob == nil {
		if call.AudioURL == nil {
			wErr(w, r, autoError(ErrNoCall))
			return
		}

		http.Redirect(w, r, call.AudioURL.String(), http.StatusFound)
		return
	}

	octetStream := "application/octet-stream"
	var ext string
	if call.AudioType == nil && call.AudioName != nil {
		ext = filepath.Ext(*call.AudioName)
		if ext != "" {
			mt := mime.TypeByExtension(ext)
			if mt != "" {
				call.AudioType = &mt
			}
		}
	}

	if call.AudioType == nil {
		call.AudioType = &octetStream
	}

	if call.AudioName == nil {
		call.AudioName = common.PtrTo(call.CallDate.Time().Format(fileNameDateFmt))
	}

	common.ContentDisposition(w.Header(), *call.AudioType, *call.AudioName, p.Download != nil)
	_, _ = w.Write(call.AudioBlob)
}

func (ca *callsAPI) getCallInfoRoute(w http.ResponseWriter, r *http.Request) {
	p := struct {
		CallID uuid.UUID `param:"call"`
	}{}
	err := decodeParams(&p, r)
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}
	ctx := r.Context()
	cs := callstore.FromCtx(ctx)

	ci, err := cs.Call(ctx, p.CallID)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	respond(w, r, ci)
}

func (ca *callsAPI) getCallInfo(ctx context.Context, id ID) (SharedItem, error) {
	cs := callstore.FromCtx(ctx)
	return cs.Call(ctx, id.(uuid.UUID))
}

func (ca *callsAPI) shareCallRoute(id ID, w http.ResponseWriter, r *http.Request) {
	p := getAudioParams{
		CallID: common.PtrTo(id.(uuid.UUID)),
	}

	ca.getAudio(p, w, r)
}

func (ca *callsAPI) shareCallDLRoute(id ID, w http.ResponseWriter, r *http.Request) {
	p := getAudioParams{
		CallID:   common.PtrTo(id.(uuid.UUID)),
		Download: common.PtrTo("download"),
	}

	ca.getAudio(p, w, r)
}

func (ca *callsAPI) listCalls(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cSt := callstore.FromCtx(ctx)

	var par callstore.ListCallsParams
	err := forms.Unmarshal(r, &par, forms.WithTag("json"), forms.WithAcceptBlank(), forms.WithOmitEmpty())
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	calls, count, err := cSt.Calls(ctx, par)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	res := struct {
		Calls []database.ListCallsPRow `json:"calls"`
		Count int                      `json:"count"`
	}{
		Calls: calls,
		Count: count,
	}

	respond(w, r, res)
}
