package rest

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
	"path/filepath"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/database"

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

	r.Get(`/{call:[a-f0-9-]+}`, ca.get)
	r.Get(`/{call:[a-f0-9-]+}/{download:download}`, ca.get)

	return r
}

func (ca *callsAPI) get(w http.ResponseWriter, r *http.Request) {
	p := struct {
		CallID   *uuid.UUID `param:"call"`
		Download *string    `param:"download"`
	}{}

	err := decodeParams(&p, r)
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	if p.CallID == nil {
		wErr(w, r, badRequest(ErrNoCall))
		return
	}

	ctx := r.Context()
	db := database.FromCtx(ctx)

	call, err := db.GetCallAudioByID(ctx, *p.CallID)
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
		call.AudioName = common.PtrTo(call.CallDate.Time.Format(fileNameDateFmt))
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
