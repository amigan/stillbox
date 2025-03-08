package rest

import (
	"net/http"

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

	response := struct {
		UID      users.UserID `json:"uid"`
		Username string       `json:"username"`
	}{
		UID:      user.ID,
		Username: user.Username,
	}

	respond(w, r, response)
}
