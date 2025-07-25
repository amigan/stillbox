package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"dynatron.me/x/stillbox/internal/forms"
	"dynatron.me/x/stillbox/pkg/calls/callstore"
	"github.com/go-chi/chi/v5"
	"github.com/tmaxmax/go-sse"
)

type adminAPI struct {
}

func (aa *adminAPI) Subrouter() http.Handler {
	r := chi.NewMux()

	r.Post("/move-calls", aa.moveCalls)

	return r
}

func (aa *adminAPI) moveCalls(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	r = r.WithContext(ctx)

	cst := callstore.FromCtx(ctx)

	var par callstore.MoveCallParams
	err := forms.Unmarshal(r, &par, forms.WithTag("json"), forms.WithAcceptBlank(), forms.WithOmitEmpty())
	if err != nil {
		wErr(w, r, badRequestErrText(err))
		return
	}

	s := &sse.Server{}

	progress := make(chan int64, 8)

	par.ProgressChan = progress

	go func() {
		totalCount, ok := <-progress
		if !ok {
			return
		}

		m := &sse.Message{}
		m.AppendData(fmt.Sprintf(`{"total":%d}`, totalCount))
		_ = s.Publish(m)

		for {
			select {
			case msg, ok := <-progress:
				if !ok {
					return
				}

				m := &sse.Message{}
				m.AppendData(fmt.Sprintf(`{"completed":%d}`, msg))
				_ = s.Publish(m)
			case <-ctx.Done():
				return
			}
		}
	}()

	s.Logger = func(_ *http.Request) *slog.Logger {
		return slog.Default()
	}
	go func() {
		numRows, err := cst.MoveCallAudio(ctx, par)
		if err != nil {
			es, _ := json.Marshal(map[string]string{"error": err.Error()})
			m := &sse.Message{}
			m.AppendData(string(es))
			_ = s.Publish(m)
			_ = s.Shutdown(ctx)

			return
		}

		m := &sse.Message{}
		m.AppendData(fmt.Sprintf(`{"final":%d}`, numRows))
		_ = s.Publish(m)
		_ = s.Shutdown(ctx)
	}()
	s.ServeHTTP(w, r)
}
