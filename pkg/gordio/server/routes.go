package server

import (
	"net/http"
	"time"

	"dynatron.me/x/stillbox/pkg/gordio/database"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/go-chi/render"
)

func (s *Server) setupRoutes() {
	r := s.r
	r.Use(middleware.WithValue(database.DBCTXKeyValue, s.db))

	r.Group(func(r chi.Router) {
		// authenticated routes
		r.Use(s.auth.AuthMiddleware(), s.auth.VerifyMiddleware())
	})

	r.Group(func(r chi.Router) {
		r.Use(rateLimiter())
		r.Use(render.SetContentType(render.ContentTypeJSON))
		// public routes
		r.Mount("/", s.auth.Routes())
		r.Mount("/", s.sources.PublicRoutes())
	})

	r.Group(func(r chi.Router) {
		r.Use(rateLimiter(), s.auth.VerifyMiddleware())

		// optional auth routes

		r.Get("/", s.routeIndex)
	})
}

func rateLimiter() func(http.Handler) http.Handler {
	return httprate.LimitByRealIP(100, 1*time.Minute)
}

func (s *Server) routeIndex(w http.ResponseWriter, r *http.Request) {
	if cl, authenticated := s.auth.Authenticated(r); authenticated {
		w.Write([]byte("Hello " + cl["user"].(string) + "\n"))
	}
	w.Write([]byte("Welcome to gordio\n"))
}
