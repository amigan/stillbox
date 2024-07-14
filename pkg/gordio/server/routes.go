package server

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
)

func (s *Server) setupRoutes() {
	r := s.r

	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(s.jwt))
		r.Use(jwtauth.Authenticator(s.jwt))

	})

	r.Group(func (r chi.Router) {
		// public routes
	})

	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(s.jwt))

		// optional auth routes

		r.Get("/", s.routeIndex)
	})
}

func (s *Server) routeIndex(w http.ResponseWriter, r *http.Request) {
	if s.Authenticated(r) {
		w.Write([]byte(fmt.Sprint("Welcome\n")))
		// error
	}
}

func (s *Server) Authenticated(r *http.Request) bool {
	// TODO: check IP against ACL, or conf.Public, and against map of routes
	tok, _, _ := jwtauth.FromContext(r.Context())
	return tok != nil
}
