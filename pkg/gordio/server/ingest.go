package server

import (
	"context"

	"dynatron.me/x/stillbox/pkg/gordio/calls"
	"dynatron.me/x/stillbox/pkg/gordio/database"
)

func (s *Server) Ingest(ctx context.Context, call *calls.Call) {
	// decouple this context from the one we call sinks with
	db := database.FromCtx(ctx)
	nctx := database.CtxWithDB(context.Background(), db)

	s.sinks.EmitCall(nctx, call)
}
