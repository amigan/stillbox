package rest

import (
	"net/http"
	"net/netip"

	"dynatron.me/x/stillbox/internal/forms"
	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/authn"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/users"
	"github.com/go-chi/chi/v5"
)

type usersAPI struct {
	authn authn.Authn
}

func newUsersAPI(auth authn.Authn) *usersAPI {
	return &usersAPI{
		authn: auth,
	}
}

func (ua *usersAPI) Subrouter() http.Handler {
	r := chi.NewMux()

	r.Get("/{user}", ua.getUser)
	r.Post("/passwd", ua.passwd)
	r.Get("/", ua.getUser)

	return r
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

	if p.User == "" { // self
		ctx := r.Context()
		ctxUser := authn.UsernameFrom(ctx)
		if ctxUser != nil {
			p.User = *ctxUser
		}
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
		RealName      *string         `json:"realName,omitempty"`
		Email         string          `json:"email,omitzero"`
		Roles         []string        `json:"roles,omitzero"`
		LastLoginAt   *jsontypes.Time `json:"lastLoginAt,omitempty"`
		LastLoginFrom *netip.Addr     `json:"lastLoginFrom,omitempty"`
		IsAdmin       bool            `json:"isAdmin,omitzero"`
	}{
		UID:           user.ID,
		Username:      user.Username,
		Email:         user.Email,
		RealName:      user.RealName,
		Roles:         user.Roles,
		LastLoginAt:   user.LastLoginAt,
		LastLoginFrom: user.LastLoginFrom,
		IsAdmin:       user.HasRole(entities.RoleAdmin),
	}

	respond(w, r, response)
}

func (ua *usersAPI) passwd(w http.ResponseWriter, r *http.Request) {
	inp := struct {
		Username    *string `json:"username"`
		OldPassword *string `json:"oldPassword"`
		NewPassword string  `json:"newPassword"`
	}{}

	err := forms.Unmarshal(r, &inp, forms.WithTag("json"), forms.WithAcceptBlank(), forms.WithOmitEmpty())
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	ctx := r.Context()
	err = ua.authn.ChangePassword(ctx, inp.Username, inp.OldPassword, inp.NewPassword)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}
}
