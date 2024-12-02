package server

import (
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"dynatron.me/x/stillbox/client"
	"dynatron.me/x/stillbox/internal/version"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/go-chi/render"
)

const (
	serverHeader = "Server"
)

func (s *Server) setupRoutes() {
	clientRoot, err := fs.Sub(client.Client, client.Prefix)
	if err != nil {
		panic(err)
	}

	r := s.r
	r.Use(middleware.WithValue(database.DBCtxKey, s.db))
	r.Use(middleware.WithValue(tgstore.StoreCtxKey, s.tgs))

	s.installPprof()

	r.Group(func(r chi.Router) {
		// authenticated routes
		r.Use(s.auth.VerifyMiddleware(), s.auth.AuthMiddleware())
		s.nex.PrivateRoutes(r)
		s.auth.PrivateRoutes(r)
		s.alerter.PrivateRoutes(r)
		r.Mount("/api", s.rest.Subrouter())
	})

	r.Group(func(r chi.Router) {
		s.rateLimit(r)
		r.Use(render.SetContentType(render.ContentTypeJSON))
		// public routes
		s.auth.PublicRoutes(r)
		s.sources.PublicRoutes(r)
	})

	r.Group(func(r chi.Router) {
		s.rateLimit(r)
		r.Use(s.auth.VerifyMiddleware())

		// optional auth routes

		s.clientRoute(r, clientRoot)
	})
}

func (s *Server) rateLimit(r chi.Router) {
	if s.conf.RateLimit.Verify() {
		r.Use(rateLimiter(&s.conf.RateLimit))
	}
}
func rateLimiter(cfg *config.RateLimit) func(http.Handler) http.Handler {
	return httprate.LimitByRealIP(cfg.Requests, cfg.Over)
}

func (s *Server) clientRoute(r chi.Router, clientRoot fs.FS) {
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		hfs := http.FS(clientRoot)
		var pe *fs.PathError

		pc := path.Clean(r.URL.Path)
		f, err := hfs.Open(pc)
		if err != nil {
			if errors.As(err, &pe) {
				http.ServeFileFS(w, r, clientRoot, "/index.html")
				return
			}
		} else {
			f.Close()
		}

		rctx := chi.RouteContext(r.Context())
		pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
		fs := http.StripPrefix(pathPrefix, http.FileServer(http.FS(clientRoot)))
		fs.ServeHTTP(w, r)
	})
}

func ServerHeaderAdd(next http.Handler) http.Handler {
	serverString := version.HttpString(version.Name)
	hfn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(serverHeader, serverString)
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(hfn)
}
