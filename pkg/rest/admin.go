package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
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

func (aa *adminAPI) moveCalls(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	cst := callstore.FromCtx(ctx)

	var par callstore.MoveCallParams
	err := forms.Unmarshal(r, &par, forms.WithTag("json"), forms.WithAcceptBlank(), forms.WithOmitEmpty())
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	progress := make(chan int64, 8)

	par.ProgressChan = progress

	rc := http.NewResponseController(w)

	var sentSSE atomic.Bool

	go func() {
		totalCount, ok := <-progress
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
			case msg, ok := <-progress:
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

	numRows, err := cst.MoveCallAudio(ctx, par)
	if err != nil {
		if sentSSE.Load() {
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

	fmt.Fprintf(w, "data:{\"final\":%d}\n\n", numRows)
	rc.Flush()
}
