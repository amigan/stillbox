package rest

import (
	"net/http"
	"net/netip"

	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/rbac"
	"dynatron.me/x/stillbox/pkg/rbac/entities"
	"dynatron.me/x/stillbox/pkg/users"
	"github.com/go-chi/chi/v5"
)

type usersAPI struct {
}

func (ua *usersAPI) Subrouter() http.Handler {
	r := chi.NewMux()

	r.Get("/{user}", ua.getUser)

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

	ctx := r.Context()
	ust := users.FromCtx(ctx)
	user, err := ust.GetUser(ctx, p.User)
	if err != nil {
		wErr(w, r, recordNotFound(err))
		return
	}

	var lastLoginAt *jsontypes.Time
	var lastLoginFrom *netip.Addr

	// TODO: this should probably be moved into the store
	_, err = rbac.Check(ctx, user, rbac.WithActions(entities.ActionReadPrivileged))
	if err == nil {
		lastLoginAt = user.LastLoginAt
		lastLoginFrom = user.LastLoginFrom
	}

	response := struct {
		UID           users.UserID    `json:"uid"`
		Username      string          `json:"username"`
		LastLoginAt   *jsontypes.Time `json:"lastLoginAt,omitempty"`
		LastLoginFrom *netip.Addr     `json:"lastLoginFrom,omitempty"`
	}{
		UID:           user.ID,
		Username:      user.Username,
		LastLoginAt:   lastLoginAt,
		LastLoginFrom: lastLoginFrom,
	}

	respond(w, r, response)
}
