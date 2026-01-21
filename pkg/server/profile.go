//go:build pprof
// +build pprof

package server

import (
	"net/http"
	"runtime"

	"dynatron.me/x/stillbox/internal/acl"

	"github.com/felixge/fgprof"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
)

type profileMount struct {
	chi.Router

	acl *acl.IP
}

func (pm *profileMount) middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := pm.acl.Allowed(r); err != nil {
				log.Error().Err(err).Str("remote", r.RemoteAddr).Msg("pprof")
				http.Error(w, "access denied", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (s *Server) pprof(profAcl *acl.IPConfig) http.Handler {
	if profAcl == nil {
		log.Error().Msg("this is a pprof build but no profile ACL set in config! disabling profiling")
		return nil
	}

	ipacl, err := profAcl.IPACL()
	if err != nil {
		log.Error().Err(err).Msg("this is a pprof build but bad profile ACL set in config! disabling profiling")
		return nil
	}
	runtime.SetMutexProfileFraction(5)
	runtime.SetBlockProfileRate(1)
	r := chi.NewRouter()
	profMount := &profileMount{Router: r, acl: ipacl}
	r.Use(profMount.middleware())
	r.Mount("/", middleware.Profiler())
	r.Handle("/fgprof", fgprof.Handler())

	return profMount
}
