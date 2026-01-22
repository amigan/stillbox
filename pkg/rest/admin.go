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
			fmt.Fprintln(w, err)
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

	ps, progress := NewProgressSender(w, r)

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

	if !ps.Close(numRows) {
		respond(w, r, map[string]int64{"count": numRows})
	}
}
