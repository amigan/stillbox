package calls

import (
	"context"
	"fmt"
	"sync"
	"time"

	"dynatron.me/x/stillbox/pkg/gordio/database"

	"dynatron.me/x/stillbox/internal/ruletime"
	"dynatron.me/x/stillbox/internal/trending"

	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"
)

type Talkgroup struct {
	System    uint32
	Talkgroup uint32
}

func (c *Call) TalkgroupTuple() Talkgroup {
	return Talkgroup{System: uint32(c.System), Talkgroup: uint32(c.Talkgroup)}
}

func TG[T int | uint | int64 | uint64 | int32 | uint32](sys, tgid T) Talkgroup {
	return Talkgroup{
		System:    uint32(sys),
		Talkgroup: uint32(tgid),
	}
}

func (t Talkgroup) Pack() int64 {
	// P25 system IDs are 12 bits, so we can fit them in a signed 8 byte int (int64, pg INT8)
	return int64((int64(t.System) << 32) | int64(t.Talkgroup))
}

func (t Talkgroup) String() string {
	return fmt.Sprintf("%d:%d", t.System, t.Talkgroup)
}

func PackedTGs(tg []Talkgroup) []int64 {
	s := make([]int64, len(tg))

	for i, v := range tg {
		s[i] = v.Pack()
	}

	return s
}

type tgMap map[Talkgroup]database.GetTalkgroupWithLearnedByPackedIDsRow

type TalkgroupCache interface {
	TG(ctx context.Context, tg Talkgroup) (database.GetTalkgroupWithLearnedByPackedIDsRow, bool)
	SystemName(ctx context.Context, id int) (string, bool)
	ApplyAlertRules(score trending.Score[Talkgroup], t time.Time, coversOpts ...ruletime.CoversOption) float64
	Hint(ctx context.Context, tgs []Talkgroup) error
	Load(ctx context.Context, tgs []int64) error
	Invalidate()
}

func (t *talkgroupCache) Invalidate() {
	t.Lock()
	defer t.Unlock()
	clear(t.tgs)
	clear(t.systems)
	clear(t.AlertConfig)
}

type talkgroupCache struct {
	sync.RWMutex
	AlertConfig
	tgs     tgMap
	systems map[int32]string
}

func NewTalkgroupCache() TalkgroupCache {
	tgc := &talkgroupCache{
		tgs:         make(tgMap),
		systems:     make(map[int32]string),
		AlertConfig: make(AlertConfig),
	}

	return tgc
}

func (t *talkgroupCache) Hint(ctx context.Context, tgs []Talkgroup) error {
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

func (t *talkgroupCache) add(rec database.GetTalkgroupWithLearnedByPackedIDsRow) error {
	tg := TG(rec.SystemID, rec.Tgid)
	t.tgs[tg] = rec
	t.systems[rec.SystemID] = rec.SystemName

	return t.AlertConfig.AddAlertConfig(tg, rec.AlertConfig)
}

func (t *talkgroupCache) Load(ctx context.Context, tgs []int64) error {
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

func (t *talkgroupCache) TG(ctx context.Context, tg Talkgroup) (database.GetTalkgroupWithLearnedByPackedIDsRow, bool) {
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

func (t *talkgroupCache) SystemName(ctx context.Context, id int) (name string, has bool) {
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
