package server

import (
	"context"

	"dynatron.me/x/stillbox/pkg/calls"
)

func (s *Server) Ingest(ctx context.Context, call *calls.Call) error {
	ctx = context.WithoutCancel(ctx)
	return s.sinks.EmitCall(ctx, call)
}
