package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"

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

	return r
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

	rc := http.NewResponseController(w)
	progress := strings.Contains(r.Header.Get("Accept"), "text/event-stream")
	var sentSSE atomic.Bool

	if progress {
		progCh := make(chan int64, 8)
		par.ProgressChan = progCh

		go func() {
			totalCount, ok := <-progCh
			if !ok {
				return
			}

			sentSSE.Store(true)

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			fmt.Fprintf(w, "data:{\"total\":%d}\n\n", totalCount)

			rc.Flush()

			for {
				select {
				case msg, ok := <-progCh:
					if !ok {
						return
					}

					fmt.Fprintf(w, "data:{\"completed\":%d}\n\n", msg)
					rc.Flush()
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	numRows, err := cst.MoveCallAudio(ctx, par)
	if err != nil {
		if progress && sentSSE.Load() {
			b, err := json.Marshal(map[string]string{"error": err.Error()})
			if err != nil {
				log.Error().Err(err).Msg("move call rest encode")
			}
			fmt.Fprintf(w, "data:%s\n", string(b))
		} else {
			wErr(w, r, autoError(err))
		}
		return
	}

	if progress {
		fmt.Fprintf(w, "data:{\"final\":%d}\n\n", numRows)
		rc.Flush()
	} else {
		respond(w, r, map[string]int64{"count": numRows})
	}
}
