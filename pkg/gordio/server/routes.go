package server

import (
	"net/http"
	"time"

	"dynatron.me/x/stillbox/pkg/gordio/database"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
)

func (s *Server) setupRoutes() {
	r := s.r
	r.Use(middleware.WithValue(database.DBCTXKeyValue, s.db))

	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(s.jwt))
		r.Use(jwtauth.Authenticator(s.jwt))

	})

	r.Group(func(r chi.Router) {
		r.Use(rateLimiter())
		r.Use(render.SetContentType(render.ContentTypeJSON))
		// public routes
		r.Post("/auth", s.routeAuth)
	})

	r.Group(func(r chi.Router) {
		r.Use(rateLimiter())
		r.Use(jwtauth.Verifier(s.jwt))

		// optional auth routes

		r.Get("/", s.routeIndex)
	})
}

func rateLimiter() func(http.Handler) http.Handler {
	return httprate.LimitByRealIP(100, 1*time.Minute)
}

func (s *Server) routeIndex(w http.ResponseWriter, r *http.Request) {
	if cl, authenticated := s.Authenticated(r); authenticated {
		w.Write([]byte("Hello " + cl["user"].(string) + "\n"))
	}
	w.Write([]byte("Welcome to gordio\n"))
}

func (s *Server) routeAuth(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	username, password := r.PostFormValue("username"), r.PostFormValue("password")
	if username == "" || password == "" {
		http.Error(w, "blank credentials", http.StatusBadRequest)
		return
	}

	tok, err := s.Login(r.Context(), username, password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	jr := struct {
		JWT string `json:"jwt"`
	}{
		JWT: tok,
	}
	render.JSON(w, r, &jr)
}
