package server

import (
	"context"

	"dynatron.me/x/stillbox/pkg/calls"
)

func (s *Server) Ingest(ctx context.Context, call *calls.Call) error {
	return s.sinks.EmitCall(context.Background(), call)
}
