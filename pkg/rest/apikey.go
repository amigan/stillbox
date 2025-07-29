package rest

import (
	"net/http"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/forms"
	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/users"
	"github.com/go-chi/chi/v5"
)

type apiKeyAPI struct {
}

func (aa *apiKeyAPI) Subrouter() http.Handler {
	r := chi.NewMux()

	r.Post("/create", aa.createAPIkey)

	return r
}

func (*apiKeyAPI) createAPIkey(w http.ResponseWriter, r *http.Request) {
	input := struct {
		Owner     *int            `json:"owner"`
		Name      *string         `json:"name"`
		ExpiresAt *jsontypes.Time `json:"expiresAt"`
		Disabled  *bool           `json:"disabled"`
	}{}

	err := forms.Unmarshal(r, &input, forms.WithTag("json"), forms.WithAcceptBlank(), forms.WithOmitEmpty())
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	ctx := r.Context()
	ust := users.FromCtx(ctx)
	key, err := ust.CreateAPIKey(ctx, (*users.UserID)(input.Owner), input.Name, (*time.Time)(input.ExpiresAt), common.ZeroIfNil(input.Disabled))
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	respond(w, r, key)

}
