//go:build !pprof
// +build !pprof

package server

import (
	"net/http"

	"dynatron.me/x/stillbox/internal/acl"
)

func (s *Server) pprof(_ *acl.IPConfig) http.Handler { return nil }
