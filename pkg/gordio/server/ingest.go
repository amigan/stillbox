package server

import (
	"context"

	"dynatron.me/x/stillbox/pkg/gordio/calls"
)

func (s *Server) Ingest(ctx context.Context, call *calls.Call) {
	s.sinks.EmitCall(context.Background(), call)
}
