package server

import (
	"context"

	"dynatron.me/x/stillbox/pkg/calls"

	"github.com/rs/zerolog/log"
)

func (s *Server) Ingest(ctx context.Context, call *calls.Call) {
	err := call.ComputeLength()
	if err != nil {
		log.Error().Err(err).Msg("compute length failed")
	}

	s.sinks.EmitCall(context.Background(), call)
}
