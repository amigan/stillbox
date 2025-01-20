package callstore

import (
	"context"
	"fmt"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/jsontypes"

	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/rbac"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
	"dynatron.me/x/stillbox/pkg/users"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Store interface {
	// AddCall adds a call to the database.
	AddCall(ctx context.Context, call *calls.Call) error

	// DeleteCall deletes a call.
	Delete(ctx context.Context, id uuid.UUID) error

	// CallAudio returns a CallAudio struct
	CallAudio(ctx context.Context, id uuid.UUID) (*calls.CallAudio, error)

	// Call returns the call's metadata.
	Call(ctx context.Context, id uuid.UUID) (*calls.Call, error)

	// Calls gets paginated Calls.
	Calls(ctx context.Context, p CallsParams) (calls []database.ListCallsPRow, totalCount int, err error)
}

type store struct {
	db database.Store
}

func NewStore(db database.Store) *store {
	return &store{
		db: db,
	}
}

type storeCtxKey string

const StoreCtxKey storeCtxKey = "store"

func CtxWithStore(ctx context.Context, s Store) context.Context {
	return context.WithValue(ctx, StoreCtxKey, s)
}

func FromCtx(ctx context.Context) Store {
	s, ok := ctx.Value(StoreCtxKey).(Store)
	if !ok {
		panic("no call store in context")
	}

	return s
}

func toAddCallParams(call *calls.Call) database.AddCallParams {
	return database.AddCallParams{
		ID:          call.ID,
		Submitter:   call.Submitter.Int32Ptr(),
		System:      call.System,
		Talkgroup:   call.Talkgroup,
		CallDate:    pgtype.Timestamptz{Time: call.DateTime, Valid: true},
		AudioName:   common.NilIfZero(call.AudioName),
		AudioBlob:   call.Audio,
		AudioType:   common.NilIfZero(call.AudioType),
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

func (s *store) AddCall(ctx context.Context, call *calls.Call) error {
	_, err := rbac.Check(ctx, call, rbac.WithActions(rbac.ActionCreate))
	if err != nil {
		return err
	}

	params := toAddCallParams(call)
	db := database.FromCtx(ctx)
	tgs := tgstore.FromCtx(ctx)

	err = db.InTx(ctx, func(tx database.Store) error {
		err := tx.AddCall(ctx, params)
		if err != nil {
			return fmt.Errorf("add call: %w", err)
		}

		return nil
	}, pgx.TxOptions{})

	if err != nil && database.IsTGConstraintViolation(err) {
		return db.InTx(ctx, func(tx database.Store) error {
			_, err := tgs.LearnTG(ctx, call)
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

	return nil
}

func (s *store) CallAudio(ctx context.Context, id uuid.UUID) (*calls.CallAudio, error) {
	_, err := rbac.Check(ctx, rbac.UseResource(rbac.ResourceCall), rbac.WithActions(rbac.ActionRead))
	if err != nil {
		return nil, err
	}

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

func (s *store) Call(ctx context.Context, id uuid.UUID) (*calls.Call, error) {
	_, err := rbac.Check(ctx, rbac.UseResource(rbac.ResourceCall), rbac.WithActions(rbac.ActionRead))
	if err != nil {
		return nil, err
	}

	db := database.FromCtx(ctx)

	c, err := db.GetCall(ctx, id)
	if err != nil {
		return nil, err
	}

	var sub *users.UserID
	if c.Submitter != nil {
		sub = common.PtrTo(users.UserID(*c.Submitter))
	}

	return &calls.Call{
		ID:             c.ID,
		Submitter:      sub,
		System:         c.System,
		Talkgroup:      c.Talkgroup,
		DateTime:       c.CallDate.Time,
		AudioName:      common.ZeroIfNil(c.AudioName),
		AudioType:      common.ZeroIfNil(c.AudioType),
		AudioURL:       c.AudioUrl,
		Duration:       calls.CallDuration(time.Duration(common.ZeroIfNil(c.Duration)) * time.Millisecond),
		Frequency:      c.Frequency,
		Frequencies:    c.Frequencies,
		Patches:        c.Patches,
		TalkgroupLabel: c.TGLabel,
		TalkgroupGroup: c.TGGroup,
		TGAlphaTag:     c.TGAlphaTag,
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
	_, err = rbac.Check(ctx, rbac.UseResource(rbac.ResourceCall), rbac.WithActions(rbac.ActionRead))
	if err != nil {
		return nil, 0, err
	}

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
			Start:      par.Start,
			End:        par.End,
			TagsAny:    par.TagsAny,
			TagsNot:    par.TagsNot,
			TGFilter:   par.TGFilter,
			LongerThan: par.LongerThan,
		})
		if err != nil {
			return err
		}

		if offset > int32(count) {
			return common.ErrPageOutOfRange
		}

		rows, err = db.ListCallsP(ctx, par)
		return err
	}, pgx.TxOptions{})
	if txErr != nil {
		return nil, 0, txErr
	}

	return rows, int(count), err
}

func (s *store) Delete(ctx context.Context, id uuid.UUID) error {
	callOwn, err := s.getCallOwner(ctx, id)
	if err != nil {
		return err
	}

	_, err = rbac.Check(ctx, &callOwn, rbac.WithActions(rbac.ActionDelete))
	if err != nil {
		return err
	}

	return database.FromCtx(ctx).DeleteCall(ctx, id)
}

func (s *store) getCallOwner(ctx context.Context, id uuid.UUID) (calls.Call, error) {
	subInt, err := database.FromCtx(ctx).GetCallSubmitter(ctx, id)

	var sub *users.UserID

	if subInt != nil {
		sub = common.PtrTo(users.UserID(*subInt))
	}
	return calls.Call{ID: id, Submitter: sub}, err
}
