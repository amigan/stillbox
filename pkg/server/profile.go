//go:build pprof
// +build pprof

package server

import (
	"github.com/go-chi/chi/v5/middleware"
)

func (s *Server) installPprof() {
	s.r.Mount("/debug", middleware.Profiler())
}
