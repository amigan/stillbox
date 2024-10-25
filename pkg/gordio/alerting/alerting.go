package alerting

import (
	"context"
	"sort"
	"sync"
	"time"

	cl "dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/gordio/config"
	"dynatron.me/x/stillbox/pkg/gordio/database"
	"dynatron.me/x/stillbox/pkg/gordio/sinks"

	"dynatron.me/x/stillbox/internal/timeseries"
	"dynatron.me/x/stillbox/internal/trending"

	"github.com/rs/zerolog/log"
)

const (
	ScoreThreshold      = -1
	CountThreshold      = 1.0
	alerterTickInterval = time.Minute
)

type Alerter interface {
	sinks.Sink

	Enabled() bool
	Go(context.Context)

	stats
}

type alerter struct {
	sync.RWMutex
	clock     timeseries.Clock
	cfg       config.Alerting
	scorer    trending.Scorer[cl.Talkgroup]
	scores    trending.Scores[cl.Talkgroup]
	lastScore time.Time
}

type noopAlerter struct{}

type offsetClock time.Duration

func (c *offsetClock) Now() time.Time {
	return time.Now().Add(c.Duration())
}

func (c *offsetClock) Duration() time.Duration {
	return time.Duration(*c)
}

func OffsetClock(d time.Duration) offsetClock {
	return offsetClock(d)
}

type AlertOption func(*alerter)

func WithClock(clock timeseries.Clock) AlertOption {
	return func(as *alerter) {
		as.clock = clock
	}
}

func New(cfg config.Alerting, opts ...AlertOption) Alerter {
	if !cfg.Enable {
		return &noopAlerter{}
	}

	as := &alerter{
		cfg:   cfg,
		clock: timeseries.DefaultClock,
	}

	for _, opt := range opts {
		opt(as)
	}

	as.scorer = trending.NewScorer[cl.Talkgroup](
		trending.WithTimeSeries(as.newTimeSeries),
		trending.WithStorageDuration[cl.Talkgroup](time.Hour*24*time.Duration(cfg.LookbackDays)),
		trending.WithRecentDuration[cl.Talkgroup](cfg.Recent),
		trending.WithHalfLife[cl.Talkgroup](cfg.HalfLife),
		trending.WithScoreThreshold[cl.Talkgroup](ScoreThreshold),
		trending.WithCountThreshold[cl.Talkgroup](CountThreshold),
		trending.WithClock[cl.Talkgroup](as.clock),
	)

	return as
}

func (as *alerter) Go(ctx context.Context) {
	as.startBackfill(ctx)

	as.score(ctx, time.Now())
	ticker := time.NewTicker(alerterTickInterval)

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

func (as *alerter) newTimeSeries(id cl.Talkgroup) trending.TimeSeries {
	ts, _ := timeseries.NewTimeSeries(timeseries.WithGranularities(
		[]timeseries.Granularity{
			{Granularity: time.Second, Count: 60},
			{Granularity: time.Minute, Count: 10},
			{Granularity: time.Hour, Count: 24},
			{Granularity: time.Hour * 24, Count: int(as.cfg.LookbackDays)},
		},
	), timeseries.WithClock(as.clock))
	return ts
}

func (as *alerter) startBackfill(ctx context.Context) {
	now := time.Now()
	since := now.Add(-24 * time.Hour * time.Duration(as.cfg.LookbackDays))
	log.Debug().Time("since", since).Msg("starting stats backfill")
	count, err := as.backfill(ctx, since)
	if err != nil {
		log.Error().Err(err).Msg("backfill failed")
		return
	}
	log.Debug().Int("count", count).Str("in", time.Now().Sub(now).String()).Int("len", as.scorer.Score().Len()).Msg("backfill finished")
}

func (as *alerter) score(ctx context.Context, now time.Time) {
	as.Lock()
	defer as.Unlock()

	as.scores = as.scorer.Score()
	as.lastScore = now
	sort.Sort(as.scores)
}

func (as *alerter) backfill(ctx context.Context, since time.Time) (count int, err error) {
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

func (as *alerter) SinkType() string {
	return "alerting"
}

func (as *alerter) Call(ctx context.Context, call *cl.Call) error {
	as.Lock()
	defer as.Unlock()
	as.scorer.AddEvent(call.TalkgroupTuple(), call.DateTime)

	return nil
}

func (*alerter) Enabled() bool { return true }

func (*noopAlerter) SinkType() string                         { return "noopAlerter" }
func (*noopAlerter) Call(_ context.Context, _ *cl.Call) error { return nil }
func (*noopAlerter) Go(_ context.Context)                     {}
func (*noopAlerter) Enabled() bool                            { return false }
