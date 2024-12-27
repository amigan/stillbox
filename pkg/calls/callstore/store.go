package callstore

import (
	"context"
	"fmt"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/jsontypes"

	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/database"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Store interface {
	// CallAudio returns a CallAudio struct
	CallAudio(ctx context.Context, id uuid.UUID) (*calls.CallAudio, error)

	// Calls gets paginated Calls.
	Calls(ctx context.Context, p CallsParams) (calls []database.ListCallsPRow, totalCount int, err error)
}

type store struct {
}

func New() *store {
	return new(store)
}

type storeCtxKey string

const StoreCtxKey storeCtxKey = "store"

func CtxWithStore(ctx context.Context, s Store) context.Context {
	return context.WithValue(ctx, StoreCtxKey, s)
}

func FromCtx(ctx context.Context) Store {
	s, ok := ctx.Value(StoreCtxKey).(Store)
	if !ok {
		return New()
	}

	return s
}

func (s *store) CallAudio(ctx context.Context, id uuid.UUID) (*calls.CallAudio, error) {
	db := database.FromCtx(ctx)

	dbCall, err := db.GetCallAudioByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return &calls.CallAudio{
		CallDate:  jsontypes.Time(dbCall.CallDate.Time),
		AudioName: dbCall.AudioName,
		AudioType: dbCall.AudioType,
		AudioBlob: dbCall.AudioBlob,
	}, nil
}

type CallsParams struct {
	common.Pagination
	Direction *common.SortDirection `json:"dir"`

	Start          *jsontypes.Time `json:"start"`
	End            *jsontypes.Time `json:"end"`
	TagsAny        []string        `json:"tagsAny"`
	TagsNot        []string        `json:"tagsNot"`
	TGFilter       *string         `json:"tgFilter"`
	AtLeastSeconds *float32        `json:"atLeastSeconds"`
}

func (s *store) Calls(ctx context.Context, p CallsParams) (rows []database.ListCallsPRow, totalCount int, err error) {
	db := database.FromCtx(ctx)

	offset, perPage := p.Pagination.OffsetPerPage(100)
	par := database.ListCallsPParams{
		Start:     p.Start.PGTypeTSTZ(),
		End:       p.End.PGTypeTSTZ(),
		TagsAny:   p.TagsAny,
		TagsNot:   p.TagsNot,
		Offset:    offset,
		PerPage:   perPage,
		Direction: p.Direction.DirString(common.DirAsc),
		TGFilter:  p.TGFilter,
	}

	if p.AtLeastSeconds != nil {
		var n pgtype.Numeric
		if err := n.Scan(fmt.Sprint(*p.AtLeastSeconds * 1000)); err != nil {
			return nil, 0, err
		}

		par.LongerThan = n
	}

	var count int64
	txErr := db.InTx(ctx, func(db database.Store) error {
		var err error
		count, err = db.ListCallsCount(ctx, database.ListCallsCountParams{
			Start:    par.Start,
			End:      par.End,
			TagsAny:  par.TagsAny,
			TagsNot:  par.TagsNot,
			TGFilter: par.TGFilter,
		})
		if err != nil {
			return err
		}

		rows, err = db.ListCallsP(ctx, par)
		return err
	}, pgx.TxOptions{})
	if txErr != nil {
		return nil, 0, txErr
	}

	return rows, int(count), err
}
