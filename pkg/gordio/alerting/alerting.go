package alerting

import (
	"context"
	"sort"
	"sync"
	"time"

	cl "dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/gordio/database"

	"dynatron.me/x/stillbox/internal/timeseries"
	"dynatron.me/x/stillbox/internal/trending"

	"github.com/rs/zerolog/log"
)

const (
	StorageLookbackDays = 7
	HalfLife            = 30 * time.Minute
	RecentDuration      = 2 * time.Hour
	ScoreThreshold      = -1
	CountThreshold      = 1.0
	AlerterTickInterval = time.Minute
)

type Alerter struct {
	sync.RWMutex
	scorer    trending.Scorer[cl.Talkgroup]
	scores    trending.Scores[cl.Talkgroup]
	lastScore time.Time
}

type myClock struct {
	offset time.Duration
}

func (c *myClock) Now() time.Time {
	return time.Now().Add(c.offset)
}

func New() *Alerter {
	as := &Alerter{
		scorer: trending.NewScorer[cl.Talkgroup](
			trending.WithTimeSeries(newTimeSeries),
			trending.WithStorageDuration[cl.Talkgroup](StorageLookbackDays*24*time.Hour),
			trending.WithRecentDuration[cl.Talkgroup](RecentDuration),
			trending.WithHalfLife[cl.Talkgroup](HalfLife),
			trending.WithScoreThreshold[cl.Talkgroup](ScoreThreshold),
			trending.WithCountThreshold[cl.Talkgroup](CountThreshold),
		),
	}

	return as
}

func (as *Alerter) Go(ctx context.Context) {
	as.startBackfill(ctx)

	as.score(ctx, time.Now())
	ticker := time.NewTicker(AlerterTickInterval)

	for {
		select {
		case now := <-ticker.C:
			as.score(ctx, now)
		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}

}

func newTimeSeries(id cl.Talkgroup) trending.TimeSeries {
	ts, _ := timeseries.NewTimeSeries(timeseries.WithGranularities(
		[]timeseries.Granularity{
			{Granularity: time.Second, Count: 60},
			{Granularity: time.Minute, Count: 10},
			{Granularity: time.Hour, Count: 24},
			{Granularity: time.Hour * 24, Count: StorageLookbackDays},
		},
	))
	return ts
}

func (as *Alerter) startBackfill(ctx context.Context) {
	now := time.Now()
	since := now.Add(StorageLookbackDays * -24 * time.Hour)
	log.Debug().Time("since", since).Msg("starting stats backfill")
	count, err := as.backfill(ctx, since)
	if err != nil {
		log.Error().Err(err).Msg("backfill failed")
		return
	}
	log.Debug().Int("count", count).Str("in", time.Now().Sub(now).String()).Int("len", as.scorer.Score().Len()).Msg("backfill finished")
}

func (as *Alerter) score(ctx context.Context, now time.Time) {
	as.Lock()
	defer as.Unlock()

	as.scores = as.scorer.Score()
	as.lastScore = now
	sort.Sort(as.scores)
}

func (as *Alerter) backfill(ctx context.Context, since time.Time) (count int, err error) {
	db := database.FromCtx(ctx)
	const backfillStatsQuery = `SELECT system, talkgroup, call_date FROM calls WHERE call_date > $1 AND call_date < $2`

	rows, err := db.Query(ctx, backfillStatsQuery, since, timeseries.DefaultClock.Now())
	if err != nil {
		return count, err
	}
	defer rows.Close()

	as.Lock()
	defer as.Unlock()

	for rows.Next() {
		var tg cl.Talkgroup
		var callDate time.Time
		if err := rows.Scan(&tg.System, &tg.Talkgroup, &callDate); err != nil {
			return count, err
		}
		as.scorer.AddEvent(tg, callDate)
		count++
	}

	if err := rows.Err(); err != nil {
		return count, err
	}

	return count, nil
}

func (as *Alerter) SinkType() string {
	return "alerting"
}

func (as *Alerter) Call(ctx context.Context, call *cl.Call) error {
	as.Lock()
	defer as.Unlock()
	as.scorer.AddEvent(call.TalkgroupTuple(), call.DateTime)

	return nil
}
