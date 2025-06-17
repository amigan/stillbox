package rest

import (
	"fmt"
	"net/http"

	"dynatron.me/x/stillbox/internal/forms"
	"dynatron.me/x/stillbox/pkg/calls/callstore"
	"github.com/go-chi/chi/v5"
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
		wErr(w, r, badRequestErrText(err))
		return
	}

	progress := make(chan int64, 8)

	par.ProgressChan = progress

	flush := w.(http.Flusher)

	go func() {
		totalCount, ok := <-progress
		if !ok {
			return
		}

		fmt.Fprintf(w, "data:{\"total\":%d}\n", totalCount)

		flush.Flush()

		for {
			select {
			case msg, ok := <-progress:
				if !ok {
					return
				}

				fmt.Fprintf(w, "data:{\"completed\":%d}\n", msg)
				flush.Flush()
			case <-ctx.Done():
				return
			}
		}
	}()

	numRows, err := cst.MoveCallAudio(ctx, par)
	if err != nil {
		wErr(w, r, internalErrorErrText(err))
		return
	}

	// setup SSE; maybe we will use go-sse someday
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	fmt.Fprintf(w, "data:{\"final\":%d}\n", numRows)
	flush.Flush()
}
