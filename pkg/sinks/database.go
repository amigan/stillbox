package sinks

import (
	"context"
	"fmt"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/database"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
)

type DatabaseSink struct {
	db database.DB
}

func NewDatabaseSink(db database.DB) *DatabaseSink {
	return &DatabaseSink{db: db}
}

func (s *DatabaseSink) Call(ctx context.Context, call *calls.Call) error {
	if !call.ShouldStore() {
		log.Debug().Str("call", call.String()).Msg("received dontStore call")
		return nil
	}

	err := s.db.AddCall(ctx, s.toAddCallParams(call))
	if err != nil {
		return fmt.Errorf("add call: %w", err)
	}

	log.Debug().Str("id", call.ID.String()).Int("system", call.System).Int("tgid", call.Talkgroup).Msg("stored")

	return nil
}

func (s *DatabaseSink) SinkType() string {
	return "database"
}

func (s *DatabaseSink) toAddCallParams(call *calls.Call) database.AddCallParams {
	return database.AddCallParams{
		ID:          call.ID,
		Submitter:   call.Submitter.Int32Ptr(),
		System:      call.System,
		Talkgroup:   call.Talkgroup,
		CallDate:    pgtype.Timestamptz{Time: call.DateTime, Valid: true},
		AudioName:   common.PtrOrNull(call.AudioName),
		AudioBlob:   call.Audio,
		AudioType:   common.PtrOrNull(call.AudioType),
		Duration:    call.Duration.MsInt32Ptr(),
		Frequency:   call.Frequency,
		Frequencies: call.Frequencies,
		Patches:     call.Patches,
		TgLabel:     call.TalkgroupLabel,
		TgAlphaTag:  call.TGAlphaTag,
		TgGroup:     call.TalkgroupGroup,
		Source:      call.Source,
	}
}
