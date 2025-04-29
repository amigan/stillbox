package sinks

import (
	"context"

	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/calls/callstore"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"

	"github.com/rs/zerolog/log"
)

type DatabaseSink struct {
	db  database.Store
	tgs tgstore.Store
}

func NewDatabaseSink(store database.Store, tgs tgstore.Store) *DatabaseSink {
	return &DatabaseSink{store, tgs}
}

func (s *DatabaseSink) Name() string {
	return s.SinkType()
}

func (s *DatabaseSink) Call(ctx context.Context, call *calls.Call) error {
	if !call.ShouldStore() {
		log.Debug().Str("call", call.String()).Msg("received dontStore call")
		return nil
	}

	return callstore.FromCtx(ctx).AddCall(ctx, call)
}

func (s *DatabaseSink) SinkType() string {
	return "database"
}
