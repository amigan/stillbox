package sinks

import (
	"context"
	"fmt"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/gordio/calls"
	"dynatron.me/x/stillbox/pkg/gordio/database"

	"github.com/rs/zerolog/log"
)

type DatabaseSink struct {
	db *database.DB
}

func NewDatabaseSink(db *database.DB) *DatabaseSink {
	return &DatabaseSink{
		db: db,
	}
}

func (s *DatabaseSink) Call(ctx context.Context, call *calls.Call) error {
	db := database.FromCtx(ctx)

	dbCall, err := db.AddCall(ctx, s.toAddCallParams(call))
	if err != nil {
		return fmt.Errorf("add call: %w", err)
	}

	log.Debug().Str("id", dbCall.String()).Int("system", call.System).Int("tgid", call.Talkgroup).Msg("stored")

	return nil
}

func (s *DatabaseSink) SinkType() string {
	return "database"
}

func (s *DatabaseSink) toAddCallParams(call *calls.Call) database.AddCallParams {
	return database.AddCallParams{
		Submitter:   call.Submitter.Int32Ptr(),
		System:      call.System,
		Talkgroup:   call.Talkgroup,
		CallDate:    call.DateTime,
		AudioName:   common.PtrOrNull(call.AudioName),
		AudioBlob:   call.Audio,
		AudioType:   common.PtrOrNull(call.AudioType),
		Frequency:   call.Frequency,
		Frequencies: call.Frequencies,
		Patches:     call.Patches,
		TgLabel:     call.TalkgroupLabel,
		TgTag:       call.TalkgroupTag,
		TgGroup:     call.TalkgroupGroup,
		Source:      call.Source,
	}

}
