package rest

import (
	"net/http"

	"dynatron.me/x/stillbox/pkg/alerting/alertstore"
	"github.com/go-chi/chi/v5"
)

type alertsAPI struct {
}

func (aa *alertsAPI) Subrouter() http.Handler {
	r := chi.NewMux()

	r.Get(`/{id:[a-zA-Z0-9]+}`, aa.getAlert)
	return r
}

func (aa *alertsAPI) getAlert(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	params := struct {
		ID string `param:"id"`
	}{}

	err := decodeParams(&params, r)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	as := alertstore.FromCtx(ctx)

	alert, err := as.GetAlert(ctx, params.ID)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	respond(w, r, alert)
}
