package tgstore

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/auth"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/database"
	tgsp "dynatron.me/x/stillbox/pkg/talkgroups"

	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"
)

type tgMap map[tgsp.ID]*tgsp.Talkgroup

var (
	ErrNotFound       = errors.New("talkgroup not found")
	ErrNoSuchSystem   = errors.New("no such system")
	ErrInvalidOrderBy = errors.New("invalid pagination orderBy value")
	ErrReference      = errors.New("item is still referenced, cannot delete")
	ErrBadOrder       = errors.New("invalid order")
	ErrBadDirection   = errors.New("invalid direction")
)

type Store interface {
	// UpdateTG updates a talkgroup record.
	UpdateTG(ctx context.Context, input database.UpdateTalkgroupParams) (*tgsp.Talkgroup, error)

	// UpsertTGs upserts a slice of talkgroups.
	UpsertTGs(ctx context.Context, system int, input []database.UpsertTalkgroupParams) ([]*tgsp.Talkgroup, error)

	// CreateSystem creates a new system with the specified name and ID.
	CreateSystem(ctx context.Context, id int, name string) error

	// TG retrieves a Talkgroup from the Store.
	TG(ctx context.Context, tg tgsp.ID) (*tgsp.Talkgroup, error)

	// TGs retrieves many talkgroups from the Store.
	TGs(ctx context.Context, tgs tgsp.IDs, opts ...Option) ([]*tgsp.Talkgroup, error)

	// LearnTG learns the talkgroup from a Call.
	LearnTG(ctx context.Context, call *calls.Call) (*tgsp.Talkgroup, error)

	// SystemTGs retrieves all Talkgroups associated with a System.
	SystemTGs(ctx context.Context, systemID int, opts ...Option) ([]*tgsp.Talkgroup, error)

	// DeleteTG deletes a talkgroup record.
	DeleteTG(ctx context.Context, id tgsp.ID) error

	// DeleteSystem deletes a system. The system must have no talkgroups or incidents.
	DeleteSystem(ctx context.Context, id int) error

	// SystemName retrieves a system name from the store. It returns the record and whether one was found.
	SystemName(ctx context.Context, id int) (string, bool)

	// Hint hints the Store that the provided talkgroups will be asked for.
	Hint(ctx context.Context, tgs []tgsp.ID) error

	// Load loads the provided talkgroup ID tuples into the Store.
	Load(ctx context.Context, tgs database.TGTuples) error

	// Invalidate invalidates any caching in the Store.
	Invalidate()

	// Weight returns the final weight of this talkgroup, including its static and rules-derived weight.
	Weight(ctx context.Context, id tgsp.ID, t time.Time) float64

	// Hupper
	HUP(*config.Config)
}

type options struct {
	pagination     *Pagination
	totalDest      *int
	perPageDefault int

	filter *string
}

func sOpt(opts []Option) (o options) {
	for _, opt := range opts {
		opt(&o)
	}
	return
}

type Option func(*options)

func WithPagination(p *Pagination, defPerPage int, totalDest *int) Option {
	return func(o *options) {
		o.pagination = p
		o.perPageDefault = defPerPage
		o.totalDest = totalDest
	}
}

func (p *Pagination) SortDir() (string, error) {
	order := TGOrderTGID
	dir := TGDirAsc

	if p != nil {
		if p.OrderBy != nil {
			if !p.OrderBy.IsValid() {
				return "", ErrBadOrder
			}

			order = *p.OrderBy
		}

		if p.Direction != nil {
			if !p.Direction.IsValid() {
				return "", ErrBadDirection
			}

			dir = *p.Direction
		}
	}

	return string(order) + "_" + string(dir), nil
}

func WithFilter(f *string) Option {
	return func(o *options) {
		o.filter = f
	}
}

type TGOrder string
type TGDirection string

const (
	TGOrderID    TGOrder = "id"
	TGOrderTGID  TGOrder = "tgid"
	TGOrderGroup TGOrder = "group"
	TGOrderName  TGOrder = "name"
	TGOrderAlpha TGOrder = "alpha"

	TGDirAsc  TGDirection = "asc"
	TGDirDesc TGDirection = "desc"
)

func (t *TGDirection) IsValid() bool {
	if t == nil {
		return true
	}

	switch *t {
	case TGDirAsc, TGDirDesc:
		return true
	}

	return false

}

func (t *TGOrder) IsValid() bool {
	if t == nil {
		return true
	}

	switch *t {
	case TGOrderID, TGOrderTGID, TGOrderGroup, TGOrderName, TGOrderAlpha:
		return true
	}

	return false
}

type Pagination struct {
	common.Pagination

	OrderBy   *TGOrder     `json:"orderBy"`
	Direction *TGDirection `json:"dir"`
}

type storeCtxKey string

const StoreCtxKey storeCtxKey = "store"

func CtxWithStore(ctx context.Context, s Store) context.Context {
	return context.WithValue(ctx, StoreCtxKey, s)
}

func FromCtx(ctx context.Context) Store {
	s, ok := ctx.Value(StoreCtxKey).(Store)
	if !ok {
		return NewCache()
	}

	return s
}

func (t *cache) HUP(_ *config.Config) {
	t.Invalidate()
}

func (t *cache) Invalidate() {
	t.Lock()
	defer t.Unlock()
	t.invalidate()
}

func (t *cache) invalidate() {
	clear(t.tgs)
	clear(t.systems)
}

type cache struct {
	sync.RWMutex
	tgs     tgMap
	systems map[int]string
}

// NewCache returns a new cache Store.
func NewCache() *cache {
	tgc := &cache{
		tgs:     make(tgMap),
		systems: make(map[int]string),
	}

	return tgc
}

func (t *cache) Hint(ctx context.Context, tgs []tgsp.ID) error {
	if len(tgs) < 1 {
		return nil
	}

	t.RLock()
	var toLoad database.TGTuples
	if len(t.tgs) > len(tgs)/2 { // TODO: instrument this
		for _, tg := range tgs {
			_, ok := t.tgs[tg]
			if !ok {
				toLoad.Append(tg.System, tg.Talkgroup)
			}
		}

	} else {
		toLoad[0] = make([]uint32, 0, len(tgs))
		toLoad[1] = make([]uint32, 0, len(tgs))
		for _, g := range tgs {
			toLoad.Append(g.System, g.Talkgroup)
		}
	}

	if len(toLoad[0]) > 0 {
		t.RUnlock()
		return t.Load(ctx, toLoad)
	}

	t.RUnlock()
	return nil
}

func (t *cache) get(id tgsp.ID) (*tgsp.Talkgroup, bool) {
	t.RLock()
	defer t.RUnlock()

	tg, has := t.tgs[id]

	return tg, has
}

func (t *cache) add(rec *tgsp.Talkgroup) {
	t.Lock()
	defer t.Unlock()

	t.addNoLock(rec)
}

func (t *cache) addNoLock(rec *tgsp.Talkgroup) {
	tg := tgsp.TG(rec.System.ID, rec.Talkgroup.TGID)
	t.tgs[tg] = rec
	t.systems[rec.System.ID] = rec.System.Name
}

func (t *cache) addSysNoLock(id int, name string) {
	t.systems[id] = name
}

type rowType interface {
	database.GetTalkgroupsRow | database.GetTalkgroupsWithLearnedRow |
		database.GetTalkgroupsWithLearnedBySystemRow | database.GetTalkgroupWithLearnedRow |
		database.GetTalkgroupsWithLearnedBySystemPRow | database.GetTalkgroupsWithLearnedPRow
	row
}

type row interface {
	GetTalkgroup() database.Talkgroup
	GetSystem() database.System
	GetLearned() bool
}

func rowToTalkgroup[T rowType](r T) *tgsp.Talkgroup {
	return &tgsp.Talkgroup{
		Talkgroup: r.GetTalkgroup(),
		System:    r.GetSystem(),
		Learned:   r.GetLearned(),
	}
}

func addToRowListS[T rowType](t *cache, r []*tgsp.Talkgroup, tgRecords []T) []*tgsp.Talkgroup {
	t.Lock()
	defer t.Unlock()

	for _, rec := range tgRecords {
		tg := rowToTalkgroup(rec)
		t.addNoLock(tg)

		r = append(r, tg)
	}

	return r
}

func addToRowList[T rowType](t *cache, tgRecords []T) []*tgsp.Talkgroup {
	t.Lock()
	defer t.Unlock()
	r := make([]*tgsp.Talkgroup, 0, len(tgRecords))

	for _, rec := range tgRecords {
		tg := rowToTalkgroup(rec)
		t.addNoLock(tg)

		r = append(r, tg)
	}

	return r
}

func (t *cache) TGs(ctx context.Context, tgs tgsp.IDs, opts ...Option) ([]*tgsp.Talkgroup, error) {
	db := database.FromCtx(ctx)

	r := make([]*tgsp.Talkgroup, 0, len(tgs))
	opt := sOpt(opts)
	var err error
	if tgs != nil {
		toGet := make(tgsp.IDs, 0, len(tgs))
		for _, id := range tgs {
			rec, has := t.get(id)
			if has {
				r = append(r, rec)
			} else {
				toGet = append(toGet, id)
			}
		}

		tgRecords, err := db.GetTalkgroupsWithLearnedBySysTGID(ctx, toGet.Tuples())
		if err != nil {
			return nil, err
		}
		return addToRowListS(t, r, tgRecords), nil
	}

	// get all talkgroups

	if opt.pagination != nil {
		sortDir, err := opt.pagination.SortDir()
		if err != nil {
			return nil, err
		}

		offset, perPage := opt.pagination.OffsetPerPage(opt.perPageDefault)
		var tgRecords []database.GetTalkgroupsWithLearnedPRow
		err = db.InTx(ctx, func(db database.Store) error {
			var err error
			tgRecords, err = db.GetTalkgroupsWithLearnedP(ctx, database.GetTalkgroupsWithLearnedPParams{
				Filter:  opt.filter,
				OrderBy: sortDir,
				Offset:  offset,
				PerPage: perPage,
			})
			if err != nil {
				return err
			}

			if opt.totalDest != nil {
				count, err := db.GetTalkgroupsWithLearnedCount(ctx, opt.filter)
				if err != nil {
					return err
				}

				*opt.totalDest = int(count)
			}

			return nil
		}, pgx.TxOptions{})

		if err != nil {
			return nil, err
		}

		return addToRowListS(t, r, tgRecords), nil
	}

	tgRecords, err := db.GetTalkgroupsWithLearned(ctx)
	if err != nil {
		return nil, err
	}
	return addToRowListS(t, r, tgRecords), nil
}

func (t *cache) Load(ctx context.Context, tgs database.TGTuples) error {
	tgRecords, err := database.FromCtx(ctx).GetTalkgroupsWithLearnedBySysTGID(ctx, tgs)
	if err != nil {
		return err
	}

	for _, rec := range tgRecords {
		t.add(rowToTalkgroup(rec))
	}

	return nil
}

func (t *cache) Weight(ctx context.Context, id tgsp.ID, tm time.Time) float64 {
	tg, err := t.TG(ctx, id)
	if err != nil {
		return 1.0
	}

	m := float64(tg.Weight)

	m *= tg.AlertConfig.Apply(tm)

	return float64(m)
}

func (t *cache) SystemTGs(ctx context.Context, systemID int, opts ...Option) ([]*tgsp.Talkgroup, error) {
	db := database.FromCtx(ctx)
	opt := sOpt(opts)
	var err error
	if opt.pagination != nil {
		sortDir, err := opt.pagination.SortDir()
		if err != nil {
			return nil, err
		}

		offset, perPage := opt.pagination.OffsetPerPage(opt.perPageDefault)
		var recs []database.GetTalkgroupsWithLearnedBySystemPRow
		err = db.InTx(ctx, func(db database.Store) error {
			var err error
			recs, err = db.GetTalkgroupsWithLearnedBySystemP(ctx, database.GetTalkgroupsWithLearnedBySystemPParams{
				System:  int32(systemID),
				Filter:  opt.filter,
				OrderBy: sortDir,
				Offset:  offset,
				PerPage: perPage,
			})
			if err != nil {
				return err
			}

			if opt.totalDest != nil {
				count, err := db.GetTalkgroupsWithLearnedBySystemCount(ctx, int32(systemID), opt.filter)
				if err != nil {
					return err
				}

				*opt.totalDest = int(count)
			}

			return nil
		}, pgx.TxOptions{})

		if err != nil {
			return nil, err
		}

		return addToRowList(t, recs), nil
	}

	recs, err := db.GetTalkgroupsWithLearnedBySystem(ctx, int32(systemID))
	if err != nil {
		return nil, err
	}

	return addToRowList(t, recs), nil
}

func (t *cache) TG(ctx context.Context, tg tgsp.ID) (*tgsp.Talkgroup, error) {
	rec, has := t.get(tg)

	if has {
		return rec, nil
	}

	record, err := database.FromCtx(ctx).GetTalkgroupWithLearned(ctx, int32(tg.System), int32(tg.Talkgroup))
	switch err {
	case nil:
	case pgx.ErrNoRows:
		return nil, ErrNotFound
	default:
		log.Error().Err(err).Uint32("sys", tg.System).Uint32("tg", tg.Talkgroup).Msg("TG() cache add db get")
		return nil, errors.Join(ErrNotFound, err)
	}

	t.add(rowToTalkgroup(record))

	return rowToTalkgroup(record), nil
}

func (t *cache) SystemName(ctx context.Context, id int) (name string, has bool) {
	t.RLock()
	n, has := t.systems[id]
	t.RUnlock()

	if !has {
		sys, err := database.FromCtx(ctx).GetSystemName(ctx, id)
		if err != nil {
			return "", false
		}

		t.Lock()
		t.systems[id] = sys
		t.Unlock()

		return sys, true
	}

	return n, has
}

func (t *cache) UpdateTG(ctx context.Context, input database.UpdateTalkgroupParams) (*tgsp.Talkgroup, error) {
	sysName, has := t.SystemName(ctx, int(*input.SystemID))
	if !has {
		return nil, ErrNoSuchSystem
	}
	db := database.FromCtx(ctx)
	var tg database.Talkgroup
	err := db.InTx(ctx, func(db database.Store) error {
		var oerr error
		tg, oerr = db.UpdateTalkgroup(ctx, input)
		if oerr != nil {
			return oerr
		}
		versionBatch := db.StoreTGVersion(ctx, []database.StoreTGVersionParams{{
			Submitter: auth.UIDFrom(ctx),
			TGID:      *input.TGID,
		}})
		defer versionBatch.Close()

		versionBatch.Exec(func(_ int, err error) {
			if err != nil {
				oerr = err
				return
			}
		})

		return oerr
	}, pgx.TxOptions{})

	if err != nil {
		return nil, err
	}

	record := &tgsp.Talkgroup{
		Talkgroup: tg,
		System:    database.System{ID: int(tg.SystemID), Name: sysName},
	}
	t.add(record)

	return record, nil
}

func (t *cache) DeleteSystem(ctx context.Context, id int) error {
	t.Lock()
	defer t.Unlock()

	t.invalidate()

	err := database.FromCtx(ctx).DeleteSystem(ctx, id)
	switch {
	case err == nil:
		return nil
	case database.IsSystemConstraintViolation(err):
		return ErrReference
	}

	return err
}

func (t *cache) DeleteTG(ctx context.Context, id tgsp.ID) error {
	t.Lock()
	defer t.Unlock()

	err := database.FromCtx(ctx).InTx(ctx, func(db database.Store) error {
		err := db.StoreDeletedTGVersion(ctx, common.PtrTo(int32(id.System)), common.PtrTo(int32(id.Talkgroup)), auth.UIDFrom(ctx))
		if err != nil {
			return err
		}

		return db.DeleteTalkgroup(ctx, int32(id.System), int32(id.Talkgroup))
	}, pgx.TxOptions{})

	switch {
	case err == nil:
	case database.IsTGConstraintViolation(err):
		return ErrReference
	default:
		return err
	}

	delete(t.tgs, id)

	return nil
}

func (t *cache) LearnTG(ctx context.Context, c *calls.Call) (*tgsp.Talkgroup, error) {
	db := database.FromCtx(ctx)

	sys, has := t.SystemName(ctx, c.System)
	if !has {
		return nil, ErrNoSuchSystem
	}

	tgm, err := db.AddLearnedTalkgroup(ctx, database.AddLearnedTalkgroupParams{
		SystemID: int32(c.System),
		TGID:     int32(c.Talkgroup),
		Name:     c.TalkgroupLabel,
		AlphaTag: c.TGAlphaTag,
		TGGroup:  c.TalkgroupGroup,
	})
	if err != nil {
		return nil, err
	}

	tg := &tgsp.Talkgroup{
		Talkgroup: tgm,
		System: database.System{
			ID:   c.System,
			Name: sys,
		},
		Learned: tgm.Learned,
	}

	t.add(tg)

	return tg, nil
}

func (t *cache) UpsertTGs(ctx context.Context, system int, input []database.UpsertTalkgroupParams) ([]*tgsp.Talkgroup, error) {
	db := database.FromCtx(ctx)
	sysName, hasSys := t.SystemName(ctx, system)
	if !hasSys {
		return nil, ErrNoSuchSystem
	}
	sys := database.System{
		ID:   system,
		Name: sysName,
	}

	tgs := make([]*tgsp.Talkgroup, 0, len(input))

	err := db.InTx(ctx, func(db database.Store) error {
		versionParams := make([]database.StoreTGVersionParams, 0, len(input))
		for i := range input {
			// normalize tags
			for j, tag := range input[i].Tags {
				input[i].Tags[j] = strings.ToLower(tag)
			}

			input[i].SystemID = int32(system)
			input[i].Learned = common.PtrTo(false)
		}

		var oerr error

		tgUpsertBatch := db.UpsertTalkgroup(ctx, input)
		defer tgUpsertBatch.Close()

		tgUpsertBatch.QueryRow(func(_ int, r database.Talkgroup, err error) {
			if err != nil {
				oerr = err
				return
			}
			versionParams = append(versionParams, database.StoreTGVersionParams{
				SystemID:  int32(system),
				TGID:      r.TGID,
				Submitter: auth.UIDFrom(ctx),
			})
			tgs = append(tgs, &tgsp.Talkgroup{
				Talkgroup: r,
				System:    sys,
				Learned:   r.Learned,
			})
		})

		if oerr != nil {
			return oerr
		}

		versionBatch := db.StoreTGVersion(ctx, versionParams)
		defer versionBatch.Close()

		versionBatch.Exec(func(_ int, err error) {
			if err != nil {
				oerr = err
				return
			}
		})

		return oerr
	}, pgx.TxOptions{})

	if err != nil {
		return nil, err
	}

	// update the cache
	for _, tg := range tgs {
		t.add(tg)
	}

	return tgs, nil
}

func (t *cache) CreateSystem(ctx context.Context, id int, name string) error {
	t.Lock()
	defer t.Unlock()

	t.addSysNoLock(id, name)

	return database.FromCtx(ctx).CreateSystem(ctx, id, name)
}
