package talkgroups

import (
	"context"
	"sync"
	"time"

	"dynatron.me/x/stillbox/pkg/database"

	"dynatron.me/x/stillbox/internal/ruletime"
	"dynatron.me/x/stillbox/internal/trending"

	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"
)

type tgMap map[ID]database.GetTalkgroupWithLearnedByPackedIDsRow

type Store interface {
	// TG retrieves a Talkgroup from the Store. It returns the record and whether one was found.
	TG(ctx context.Context, tg ID) (database.GetTalkgroupWithLearnedByPackedIDsRow, bool)

	// SystemName retrieves a system name from the store. It returns the record and whether one was found.
	SystemName(ctx context.Context, id int) (string, bool)

	// ApplyAlertRules applies the score's talkgroup alert rules to the call occurring at t and returns the weighted score.
	ApplyAlertRules(score trending.Score[ID], t time.Time, coversOpts ...ruletime.CoversOption) float64

	// Hint hints the Store that the provided talkgroups will be asked for.
	Hint(ctx context.Context, tgs []ID) error

	// Load loads the provided packed talkgroup IDs into the Store.
	Load(ctx context.Context, tgs []int64) error

	// Invalidate invalidates any caching in the Store.
	Invalidate()
}

func (t *cache) Invalidate() {
	t.Lock()
	defer t.Unlock()
	clear(t.tgs)
	clear(t.systems)
	clear(t.AlertConfig)
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
		AlertConfig: make(AlertConfig),
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

func (t *cache) add(rec database.GetTalkgroupWithLearnedByPackedIDsRow) error {
	tg := TG(rec.System.ID, int(rec.Talkgroup.Tgid))
	t.tgs[tg] = rec
	t.systems[int32(rec.System.ID)] = rec.System.Name

	return t.AlertConfig.AddAlertConfig(tg, rec.Talkgroup.AlertConfig)
}

func (t *cache) Load(ctx context.Context, tgs []int64) error {
	tgRecords, err := database.FromCtx(ctx).GetTalkgroupWithLearnedByPackedIDs(ctx, tgs)
	if err != nil {
		return err
	}

	t.Lock()
	defer t.Unlock()

	for _, rec := range tgRecords {
		err := t.add(rec)

		if err != nil {
			log.Error().Err(err).Msg("add alert config fail")
		}
	}

	return nil
}

func (t *cache) TG(ctx context.Context, tg ID) (database.GetTalkgroupWithLearnedByPackedIDsRow, bool) {
	t.RLock()
	rec, has := t.tgs[tg]
	t.RUnlock()

	if has {
		return rec, has
	}

	recs, err := database.FromCtx(ctx).GetTalkgroupWithLearnedByPackedIDs(ctx, []int64{tg.Pack()})
	switch err {
	case nil:
	case pgx.ErrNoRows:
		return database.GetTalkgroupWithLearnedByPackedIDsRow{}, false
	default:
		log.Error().Err(err).Msg("TG() cache add db get")
		return database.GetTalkgroupWithLearnedByPackedIDsRow{}, false
	}

	if len(recs) < 1 {
		return database.GetTalkgroupWithLearnedByPackedIDsRow{}, false
	}

	t.Lock()
	defer t.Unlock()
	err = t.add(recs[0])
	if err != nil {
		log.Error().Err(err).Msg("TG() cache add")
		return recs[0], false
	}

	return recs[0], true
}

func (t *cache) SystemName(ctx context.Context, id int) (name string, has bool) {
	n, has := t.systems[int32(id)]

	if !has {
		sys, err := database.FromCtx(ctx).GetSystemName(ctx, id)
		if err != nil {
			return "", false
		}

		return sys, true
	}

	return n, has
}
