package callstore

import (
	"context"
	"fmt"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/internal/trending"

	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/rbac"
	"dynatron.me/x/stillbox/pkg/rbac/entities"
	"dynatron.me/x/stillbox/pkg/services"
	"dynatron.me/x/stillbox/pkg/talkgroups"
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

	// CallStats gets call stats by interval.
	CallStats(ctx context.Context, interval calls.StatsInterval, start, end jsontypes.Time) (*calls.Stats, error)

	// BackfillTrending backfills call statistics into a trending scorer.
	BackfillTrending(ctx context.Context, scorer *trending.Scorer[talkgroups.ID], stepClock func(time.Time), since, until time.Time) (count int, err error)
}

type postgresStore struct {
	db database.Store
}

func NewStore(db database.Store) *postgresStore {
	return &postgresStore{
		db: db,
	}
}

type storeCtxKey string

const StoreCtxKey storeCtxKey = "store"

func CtxWithStore(ctx context.Context, s Store) context.Context {
	return services.WithValue(ctx, StoreCtxKey, s)
}

func FromCtx(ctx context.Context) Store {
	s, ok := services.Value(ctx, StoreCtxKey).(Store)
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
		AudioUrl:    call.AudioURL,
		Duration:    call.Duration.MsInt32Ptr(),
		Frequency:   call.Frequency,
		Frequencies: call.Frequencies,
		Patches:     call.Patches,
		TalkerAlias: call.TalkerAlias,
		TGLabel:     call.TalkgroupLabel,
		TGAlphaTag:  call.TGAlphaTag,
		TGGroup:     call.TalkgroupGroup,
		Source:      call.Source,
	}
}

func (s *postgresStore) AddCall(ctx context.Context, call *calls.Call) error {
	_, err := rbac.Check(ctx, call, rbac.WithActions(entities.ActionCreate))
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

func (s *postgresStore) CallAudio(ctx context.Context, id uuid.UUID) (*calls.CallAudio, error) {
	_, err := rbac.Check(ctx, &calls.Call{ID: id}, rbac.WithActions(entities.ActionRead))
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

func (s *postgresStore) Call(ctx context.Context, id uuid.UUID) (*calls.Call, error) {
	_, err := rbac.Check(ctx, &calls.Call{ID: id}, rbac.WithActions(entities.ActionRead))
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
		TalkerAlias:    c.TalkerAlias,
		TalkgroupLabel: c.TGLabel,
		TalkgroupGroup: c.TGGroup,
		TGAlphaTag:     c.TGAlphaTag,
		Transcript:     c.Transcript,
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
	SourceFilter   *string         `json:"sourceFilter"`
	AtLeastSeconds *float32        `json:"atLeastSeconds"`
	UnknownTG      bool            `json:"unknownTG"`
}

func (s *postgresStore) Calls(ctx context.Context, p CallsParams) (rows []database.ListCallsPRow, totalCount int, err error) {
	_, err = rbac.Check(ctx, rbac.UseResource(entities.ResourceCall), rbac.WithActions(entities.ActionRead))
	if err != nil {
		return nil, 0, err
	}

	db := database.FromCtx(ctx)

	offset, perPage := p.Pagination.OffsetPerPage(100)
	par := database.ListCallsPParams{
		Start:        p.Start.PGTypeTSTZ(),
		End:          p.End.PGTypeTSTZ(),
		TagsAny:      p.TagsAny,
		TagsNot:      p.TagsNot,
		Offset:       offset,
		PerPage:      perPage,
		Direction:    p.Direction.DirString(common.DirAsc),
		TGFilter:     p.TGFilter,
		SourceFilter: p.SourceFilter,
		UnknownTG:    p.UnknownTG,
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
			Start:        par.Start,
			End:          par.End,
			TagsAny:      par.TagsAny,
			TagsNot:      par.TagsNot,
			TGFilter:     par.TGFilter,
			SourceFilter: p.SourceFilter,
			LongerThan:   par.LongerThan,
			UnknownTG:    par.UnknownTG,
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

func (s *postgresStore) Delete(ctx context.Context, id uuid.UUID) error {
	callOwn, err := s.getCallOwner(ctx, id)
	if err != nil {
		return err
	}

	_, err = rbac.Check(ctx, &callOwn, rbac.WithActions(entities.ActionDelete))
	if err != nil {
		return err
	}

	return database.FromCtx(ctx).DeleteCall(ctx, id)
}

func (s *postgresStore) getCallOwner(ctx context.Context, id uuid.UUID) (calls.Call, error) {
	subInt, err := database.FromCtx(ctx).GetCallSubmitter(ctx, id)

	var sub *users.UserID

	if subInt != nil {
		sub = common.PtrTo(users.UserID(*subInt))
	}
	return calls.Call{ID: id, Submitter: sub}, err
}

func (s *postgresStore) CallStats(ctx context.Context, interval calls.StatsInterval, start, end jsontypes.Time) (*calls.Stats, error) {
	if !interval.IsValid() {
		return nil, calls.ErrInvalidInterval
	}

	cs := &calls.Stats{
		Interval: interval,
	}

	_, err := rbac.Check(ctx, cs, rbac.WithActions(entities.ActionRead))
	if err != nil {
		return nil, err
	}

	db := database.FromCtx(ctx)

	dbs, err := db.GetCallStatsByInterval(ctx, string(interval), start.PGTypeTSTZ(), end.PGTypeTSTZ())
	if err != nil {
		return nil, err
	}

	cs.Stats = make([]calls.Stat, 0, len(dbs))
	for _, st := range dbs {
		cs.Stats = append(cs.Stats, calls.Stat{
			Count: st.Count,
			Time:  jsontypes.Time(st.Date.Time),
		})
	}

	return cs, nil
}

func (s *postgresStore) BackfillTrending(ctx context.Context, scorer *trending.Scorer[talkgroups.ID], stepClock func(time.Time), since, until time.Time) (count int, err error) {
	// We can do this through stats grants
	_, err = rbac.Check(ctx, &calls.Stats{}, rbac.WithActions(entities.ActionRead))
	if err != nil {
		return 0, err
	}

	db := database.FromCtx(ctx)
	const backfillStatsQuery = `SELECT system, talkgroup, call_date FROM calls WHERE call_date > $1 AND call_date < $2 ORDER BY call_date ASC`

	rows, err := db.DB().Query(ctx, backfillStatsQuery, since, until)
	if err != nil {
		return count, err
	}
	defer rows.Close()

	for rows.Next() {
		var tg talkgroups.ID
		var callDate time.Time
		if err := rows.Scan(&tg.System, &tg.Talkgroup, &callDate); err != nil {
			return count, err
		}
		scorer.AddEvent(tg, callDate)
		if stepClock != nil { // step the simulator if it is active
			stepClock(callDate)
		}
		count++
	}

	if err := rows.Err(); err != nil {
		return count, err
	}

	return count, nil
}
