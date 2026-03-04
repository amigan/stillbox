package rest

import (
	"net/http"

	"dynatron.me/x/stillbox/internal/forms"
	"dynatron.me/x/stillbox/pkg/authn"
	"github.com/go-chi/chi/v5"
)

type apiKeyAPI struct {
	authn authn.Authn
}

func newAPIKeyAPI(auth authn.Authn) *apiKeyAPI {
	return &apiKeyAPI{authn: auth}
}

func (aa *apiKeyAPI) Subrouter() http.Handler {
	r := chi.NewMux()

	r.Post("/create", aa.createAPIkey)

	return r
}

func (a *apiKeyAPI) createAPIkey(w http.ResponseWriter, r *http.Request) {
	var input authn.CreateAPIKeyRequest

	err := forms.Unmarshal(r, &input, forms.WithTag("json"), forms.WithAcceptBlank(), forms.WithOmitEmpty())
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	ctx := r.Context()
	key, err := a.authn.CreateAPIKey(ctx, input)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	respond(w, r, key)

}
