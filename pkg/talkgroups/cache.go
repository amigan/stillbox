package talkgroups

import (
	"context"
	"errors"
	"sync"
	"time"

	"dynatron.me/x/stillbox/internal/ruletime"

	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/database"

	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"
)

type tgMap map[ID]*Talkgroup

var (
	ErrNotFound     = errors.New("talkgroup not found")
	ErrNoSuchSystem = errors.New("no such system")
)

type Store interface {
	// UpdateTG updates a talkgroup record.
	UpdateTG(ctx context.Context, input database.UpdateTalkgroupParams) (*Talkgroup, error)

	// TG retrieves a Talkgroup from the Store.
	TG(ctx context.Context, tg ID) (*Talkgroup, error)

	// TGs retrieves many talkgroups from the Store.
	TGs(ctx context.Context, tgs IDs) ([]*Talkgroup, error)

	// SystemTGs retrieves all Talkgroups associated with a System.
	SystemTGs(ctx context.Context, systemID int32) ([]*Talkgroup, error)

	// SystemName retrieves a system name from the store. It returns the record and whether one was found.
	SystemName(ctx context.Context, id int) (string, bool)

	// ApplyAlertRules applies the score's talkgroup alert rules to the call occurring at t and returns the weighted score.
	ApplyAlertRules(id ID, t time.Time, coversOpts ...ruletime.CoversOption) float64

	// Hint hints the Store that the provided talkgroups will be asked for.
	Hint(ctx context.Context, tgs []ID) error

	// Load loads the provided packed talkgroup IDs into the Store.
	Load(ctx context.Context, tgs []int64) error

	// Invalidate invalidates any caching in the Store.
	Invalidate()

	// Weight returns the final weight of this talkgroup, including its static and rules-derived weight.
	Weight(ctx context.Context, id ID, t time.Time) float64

	// Hupper
	HUP(*config.Config)
}

type storeCtxKey string

const StoreCtxKey storeCtxKey = "store"

func CtxWithStore(ctx context.Context, s Store) context.Context {
	return context.WithValue(ctx, StoreCtxKey, s)
}

func StoreFrom(ctx context.Context) Store {
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
	clear(t.tgs)
	clear(t.systems)
	t.AlertConfig.Invalidate()
}

type cache struct {
	sync.RWMutex
	AlertConfig
	tgs     tgMap
	systems map[int32]string
}

// NewCache returns a new cache Store.
func NewCache() Store {
	tgc := &cache{
		tgs:         make(tgMap),
		systems:     make(map[int32]string),
		AlertConfig: NewAlertConfig(),
	}

	return tgc
}

func (t *cache) Hint(ctx context.Context, tgs []ID) error {
	t.RLock()
	var toLoad []int64
	if len(t.tgs) > len(tgs)/2 { // TODO: instrument this
		for _, tg := range tgs {
			_, ok := t.tgs[tg]
			if !ok {
				toLoad = append(toLoad, tg.Pack())
			}
		}

	} else {
		toLoad = make([]int64, 0, len(tgs))
		for _, g := range tgs {
			toLoad = append(toLoad, g.Pack())
		}
	}

	if len(toLoad) > 0 {
		t.RUnlock()
		return t.Load(ctx, toLoad)
	}

	t.RUnlock()
	return nil
}

func (t *cache) add(rec *Talkgroup) error {
	t.Lock()
	defer t.Unlock()

	tg := TG(rec.System.ID, rec.Talkgroup.Tgid)
	t.tgs[tg] = rec
	t.systems[int32(rec.System.ID)] = rec.System.Name

	t.AlertConfig.Add(tg, rec.AlertConfig)

	return nil
}

type row interface {
	database.GetTalkgroupsWithLearnedByPackedIDsRow | database.GetTalkgroupsWithLearnedRow |
		database.GetTalkgroupsWithLearnedBySystemRow
	GetTalkgroup() database.Talkgroup
	GetSystem() database.System
	GetLearned() bool
}

func rowToTalkgroup[T row](r T) *Talkgroup {
	return &Talkgroup{
		Talkgroup: r.GetTalkgroup(),
		System:    r.GetSystem(),
		Learned:   r.GetLearned(),
	}
}

func addToRowList[T row](t *cache, r []*Talkgroup, tgRecords []T) ([]*Talkgroup, error) {
	for _, rec := range tgRecords {
		tg := rowToTalkgroup(rec)
		err := t.add(tg)
		if err != nil {
			return nil, err
		}

		r = append(r, tg)
	}

	return r, nil
}

func (t *cache) TGs(ctx context.Context, tgs IDs) ([]*Talkgroup, error) {
	r := make([]*Talkgroup, 0, len(tgs))
	var err error
	if tgs != nil {
		toGet := make(IDs, 0, len(tgs))
		t.RLock()
		for _, id := range tgs {
			rec, has := t.tgs[id]
			if has {
				r = append(r, rec)
			} else {
				toGet = append(toGet, id)
			}
		}
		t.RUnlock()

		tgRecords, err := database.FromCtx(ctx).GetTalkgroupsWithLearnedByPackedIDs(ctx, toGet.Packed())
		if err != nil {
			return nil, err
		}
		return addToRowList(t, r, tgRecords)
	}

	// get all talkgroups

	tgRecords, err := database.FromCtx(ctx).GetTalkgroupsWithLearned(ctx)
	if err != nil {
		return nil, err
	}
	return addToRowList(t, r, tgRecords)
}

func (t *cache) Load(ctx context.Context, tgs []int64) error {
	tgRecords, err := database.FromCtx(ctx).GetTalkgroupsWithLearnedByPackedIDs(ctx, tgs)
	if err != nil {
		return err
	}

	for _, rec := range tgRecords {
		err := t.add(rowToTalkgroup(rec))

		if err != nil {
			log.Error().Err(err).Msg("add alert config fail")
		}
	}

	return nil
}

func (t *cache) Weight(ctx context.Context, id ID, tm time.Time) float64 {
	tg, err := t.TG(ctx, id)
	if err != nil {
		return 1.0
	}

	m := float64(tg.Weight)

	m *= t.AlertConfig.ApplyAlertRules(id, tm)

	return float64(m)
}

func (t *cache) SystemTGs(ctx context.Context, systemID int32) ([]*Talkgroup, error) {
	recs, err := database.FromCtx(ctx).GetTalkgroupsWithLearnedBySystem(ctx, systemID)
	if err != nil {
		return nil, err
	}

	r := make([]*Talkgroup, 0, len(recs))
	return addToRowList(t, r, recs)
}

func (t *cache) TG(ctx context.Context, tg ID) (*Talkgroup, error) {
	t.RLock()
	rec, has := t.tgs[tg]
	t.RUnlock()

	if has {
		return rec, nil
	}

	recs, err := database.FromCtx(ctx).GetTalkgroupsWithLearnedByPackedIDs(ctx, []int64{tg.Pack()})
	switch err {
	case nil:
	case pgx.ErrNoRows:
		return nil, ErrNotFound
	default:
		log.Error().Err(err).Msg("TG() cache add db get")
		return nil, errors.Join(ErrNotFound, err)
	}

	if len(recs) < 1 {
		return nil, ErrNotFound
	}

	err = t.add(rowToTalkgroup(recs[0]))
	if err != nil {
		log.Error().Err(err).Msg("TG() cache add")
		return rowToTalkgroup(recs[0]), errors.Join(ErrNotFound, err)
	}

	return rowToTalkgroup(recs[0]), nil
}

func (t *cache) SystemName(ctx context.Context, id int) (name string, has bool) {
	t.RLock()
	n, has := t.systems[int32(id)]
	t.RUnlock()

	if !has {
		sys, err := database.FromCtx(ctx).GetSystemName(ctx, id)
		if err != nil {
			return "", false
		}

		t.Lock()
		t.systems[int32(id)] = sys
		t.Unlock()

		return sys, true
	}

	return n, has
}

func (t *cache) UpdateTG(ctx context.Context, input database.UpdateTalkgroupParams) (*Talkgroup, error) {
	sysName, has := t.SystemName(ctx, int(Unpack(input.ID).System))
	if !has {
		return nil, ErrNoSuchSystem
	}

	tg, err := database.FromCtx(ctx).UpdateTalkgroup(ctx, input)
	if err != nil {
		return nil, err
	}

	record := &Talkgroup{
		Talkgroup: tg,
		System:    database.System{ID: int(tg.SystemID), Name: sysName},
	}
	t.add(record)

	return record, nil
}
