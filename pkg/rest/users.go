package rest

import (
	"net/http"
	"net/netip"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/forms"
	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/users"
	"github.com/go-chi/chi/v5"
)

type usersAPI struct {
}

func (ua *usersAPI) Subrouter() http.Handler {
	r := chi.NewMux()

	r.Get("/{user}", ua.getUser)
	r.Post("/createAPIkey", ua.createAPIkey)

	return r
}

func (ua *usersAPI) createAPIkey(w http.ResponseWriter, r *http.Request) {
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

func (ua *usersAPI) getUser(w http.ResponseWriter, r *http.Request) {
	p := struct {
		User string `param:"user"`
	}{}

	err := decodeParams(&p, r)
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	ctx := r.Context()
	ust := users.FromCtx(ctx)
	user, err := ust.GetUserPrivCheck(ctx, p.User)
	if err != nil {
		wErr(w, r, recordNotFound(err))
		return
	}

	response := struct {
		UID           users.UserID    `json:"uid"`
		Username      string          `json:"username"`
		Email         string          `json:"email,omitzero"`
		Roles         []string        `json:"roles,omitzero"`
		LastLoginAt   *jsontypes.Time `json:"lastLoginAt,omitempty"`
		LastLoginFrom *netip.Addr     `json:"lastLoginFrom,omitempty"`
	}{
		UID:           user.ID,
		Username:      user.Username,
		Email:         user.Email,
		Roles:         user.Roles,
		LastLoginAt:   user.LastLoginAt,
		LastLoginFrom: user.LastLoginFrom,
	}

	respond(w, r, response)
}
