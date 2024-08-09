package server

import (
	"io/fs"
	"net/http"
	"strings"
	"time"

	"dynatron.me/x/stillbox/client"
	"dynatron.me/x/stillbox/pkg/gordio/database"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/go-chi/render"
)

func (s *Server) setupRoutes() {
	clientRoot, err := fs.Sub(client.Calls, "calls/build/web")
	if err != nil {
		panic(err)
	}

	r := s.r
	r.Use(middleware.WithValue(database.DBCTXKeyValue, s.db))

	r.Group(func(r chi.Router) {
		// authenticated routes
		r.Use(s.auth.VerifyMiddleware(), s.auth.AuthMiddleware())
		s.nex.PrivateRoutes(r)
	})

	r.Group(func(r chi.Router) {
		r.Use(rateLimiter())
		r.Use(render.SetContentType(render.ContentTypeJSON))
		// public routes
		s.auth.PublicRoutes(r)
		s.sources.PublicRoutes(r)
	})

	r.Group(func(r chi.Router) {
		r.Use(rateLimiter(), s.auth.VerifyMiddleware())

		// optional auth routes

		s.clientRoute(r, clientRoot)
	})
}

func rateLimiter() func(http.Handler) http.Handler {
	return httprate.LimitByRealIP(100, 1*time.Minute)
}

func (s *Server) clientRoute(r chi.Router, clientRoot fs.FS) {
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.RouteContext(r.Context())
		pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
		fs := http.StripPrefix(pathPrefix, http.FileServer(http.FS(clientRoot)))
		fs.ServeHTTP(w, r)
		/*
			if cl, authenticated := s.auth.Authenticated(r); authenticated {
				w.Write([]byte("Hello " + cl["user"].(string) + "\n"))
			}
			w.Write([]byte("Welcome to gordio\n"))
		*/
	})
}
