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
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"
	"github.com/go-chi/render"
)

var (
	UserAgent = version.HttpString(version.Name)
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
	r.Use(s.WithCtxStores())

	s.installPprof()
	r.Use(s.auth.VerifyMiddleware())

	r.Group(func(r chi.Router) {
		r.Use(s.auth.AuthorizedSubjectMiddleware())
		// authenticated routes
		s.nex.PrivateRoutes(r)
		s.auth.PrivateRoutes(r)
		s.alerter.PrivateRoutes(r)
		r.Mount("/api", s.rest.Subrouter())
	})

	r.Group(func(r chi.Router) {
		s.rateLimit(r)
		r.Use(s.auth.PublicSubjectMiddleware())
		r.Use(render.SetContentType(render.ContentTypeJSON))
		// public routes
		s.sources.PublicRoutes(r)
	})

	r.Group(func(r chi.Router) {
		// auth/share routes get rate-limited heavily, but not using middleware
		s.rateLimit(r)
		r.Use(s.auth.PublicSubjectMiddleware())
		r.Use(render.SetContentType(render.ContentTypeJSON))
		s.auth.PublicRoutes(r)
		r.Mount("/share", s.rest.ShareRouter())
	})

	r.Group(func(r chi.Router) {
		s.rateLimit(r)
		// optional auth routes
		r.Use(s.auth.PublicSubjectMiddleware())

		s.clientRoute(r, clientRoot)
	})
}

// WithCtxStores is a middleware that installs all stores in the request context.
func (s *Server) WithCtxStores() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(s.fillCtx(r.Context()))
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

func (s *Server) rateLimit(r chi.Router) {
	if s.conf.Server.RateLimit.Verify() {
		r.Use(rateLimiter(&s.conf.Server.RateLimit))
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
	hfn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(serverHeader, UserAgent)
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(hfn)
}
