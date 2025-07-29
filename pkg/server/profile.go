//go:build pprof
// +build pprof

package server

import (
	"runtime"

	"github.com/felixge/fgprof"
	"github.com/go-chi/chi/v5/middleware"
)

func (s *Server) installPprof() {
	runtime.SetMutexProfileFraction(5)
	runtime.SetBlockProfileRate(1)
	s.r.Handle("/fgprof", fgprof.Handler())
	s.r.Mount("/debug", middleware.Profiler())
}
