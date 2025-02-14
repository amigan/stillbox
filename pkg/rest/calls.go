package rest

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"path/filepath"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/forms"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/calls/callstore"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/stats"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const (
	fileNameDateFmt = "2006-01-02_150405"
)

var (
	ErrNoCall = errors.New("no call specified")
)

type callsAPI struct {
}

func (ca *callsAPI) Subrouter() http.Handler {
	r := chi.NewMux()

	r.Get(`/{call:[a-f0-9-]+}`, ca.getAudioRoute)
	r.Get(`/{call:[a-f0-9-]+}/{download:download}`, ca.getAudioRoute)
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

	call, err := calls.CallAudio(ctx, *p.CallID)
	if err != nil {
		wErr(w, r, autoError(err))
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

	disposition := "inline"
	if p.Download != nil {
		disposition = "attachment"
	}

	w.Header().Add("Content-Type", *call.AudioType)
	w.Header().Add("Content-Disposition",
		fmt.Sprintf(`%s; filename="%s"`, disposition, *call.AudioName))

	_, _ = w.Write(call.AudioBlob)
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

	var par callstore.CallsParams
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
