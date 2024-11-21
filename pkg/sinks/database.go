package sinks

import (
	"context"
	"fmt"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
)

type DatabaseSink struct {
	db database.Store
	tgs tgstore.Store
}

func NewDatabaseSink(store database.Store, tgs tgstore.Store) *DatabaseSink {
	return &DatabaseSink{store, tgs}
}

func (s *DatabaseSink) Call(ctx context.Context, call *calls.Call) error {
	if !call.ShouldStore() {
		log.Debug().Str("call", call.String()).Msg("received dontStore call")
		return nil
	}

	params := s.toAddCallParams(call)

	err := s.db.InTx(ctx, func(tx database.Store) error {
		err := tx.AddCall(ctx, params)
		if err != nil {

			return fmt.Errorf("add call: %w", err)
		}

		log.Debug().Str("id", call.ID.String()).Int("system", call.System).Int("tgid", call.Talkgroup).Msg("stored")

		return nil
	}, pgx.TxOptions{})

	if err != nil && database.IsTGConstraintViolation(err) {
		return s.db.InTx(ctx, func(tx database.Store) error {
			_, err := s.tgs.LearnTG(ctx, call)
			if err != nil {
				return fmt.Errorf("learn tg: %w", err)
			}

			err = tx.AddCall(ctx, params)
			if err != nil {
				return fmt.Errorf("learn tg retry: %w", err)
			}

			return nil
		}, pgx.TxOptions{})
	}

	return err
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
		TGLabel:     call.TalkgroupLabel,
		TGAlphaTag:  call.TGAlphaTag,
		TGGroup:     call.TalkgroupGroup,
		Source:      call.Source,
	}
}
