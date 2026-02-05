package rest

import (
	"fmt"
	"net/http"

	"dynatron.me/x/stillbox/internal/forms"
	"dynatron.me/x/stillbox/pkg/calls/callstore"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

type adminAPI struct {
}

func (aa *adminAPI) Subrouter() http.Handler {
	r := chi.NewMux()

	r.Post("/move-calls", aa.moveCalls)
	r.Post("/callsgc", aa.runJournalGC)
	r.Post("/callsfsck", aa.runFsck)

	return r
}

func (*adminAPI) runJournalGC(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cst := callstore.FromCtx(ctx)
	errCh := make(chan error)
	defer close(errCh)

	go func() {
		errd := false
		for err := range errCh {
			if !errd {
				w.WriteHeader(http.StatusInternalServerError)
				errd = true
			}
			_, _ = fmt.Fprintln(w, err)
		}
	}()
	cst.DoGC(ctx, errCh)
}

// moveCalls handles the admin call move endpoint.
// If `text/event-stream` appears in the Accept: header, server-side events will be used to indicate
// progress to the client.
func (*adminAPI) moveCalls(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	cst := callstore.FromCtx(ctx)

	var par callstore.MoveCallParams
	err := forms.Unmarshal(r, &par, forms.WithTag("json"), forms.WithAcceptBlank(), forms.WithOmitEmpty())
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	ps, progress := NewProgressSender[callstore.MoveProgressMsg](w, r)

	par.ProgressChan = ps.Chan()

	numRows, err := cst.MoveCallAudio(ctx, par)
	if err != nil {
		var errSent bool
		var nerr error
		if progress {
			errSent, nerr = ps.SendErr(err)
			if nerr != nil {
				log.Error().Err(nerr).Msg("move call rest encode")
			}
		}

		if !errSent {
			wErr(w, r, autoError(err))
			return
		}
	}

	finalMsg := callstore.MoveProgressMsg{Final: &numRows}
	if !ps.Close(finalMsg) {
		respond(w, r, finalMsg)
	}
}

func (*adminAPI) runFsck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	cst := callstore.FromCtx(ctx)

	var par callstore.FsckParams
	err := forms.Unmarshal(r, &par, forms.WithTag("json"), forms.WithAcceptBlank(), forms.WithOmitEmpty())
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	ps, progress := NewProgressSender[callstore.FsckReport](w, r)
	defer ps.ConnClosed()

	par.ProgressChan = ps.Chan()

	result, err := cst.Fsck(ctx, par)
	if err != nil {
		var errSent bool
		var nerr error
		if progress {
			errSent, nerr = ps.SendErr(err)
			if nerr != nil {
				log.Error().Err(nerr).Msg("fsck call sse send error")
			}
		}

		if !errSent {
			wErr(w, r, autoError(err))
			return
		}
	}

	if progress {
		par.ProgressChan <- result
	}

	if !ps.Close(result) {
		respond(w, r, result)
	}
}
