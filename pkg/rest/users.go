package rest

import (
	"net/http"

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
}
