package alerting

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"sort"
	"sync"
	"time"

	"dynatron.me/x/stillbox/pkg/alerting/alert"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/notify"
	"dynatron.me/x/stillbox/pkg/rbac/entities"
	"dynatron.me/x/stillbox/pkg/sinks"
	"dynatron.me/x/stillbox/pkg/talkgroups"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"

	"dynatron.me/x/stillbox/internal/timeseries"
	"dynatron.me/x/stillbox/internal/trending"

	"github.com/rs/zerolog/log"
)

const (
	ScoreThreshold      = -1
	CountThreshold      = 1.0
	NotificationSubject = "Stillbox Alert"
	DefaultRenotify     = 30 * time.Minute
	alerterTickInterval = time.Minute
)

type Alerter interface {
	sinks.Sink

	Enabled() bool
	Go(context.Context)
	HUP(*config.Config)

	stats
}

type alerter struct {
	sync.RWMutex
	clock      timeseries.Clock
	cfg        config.Alerting
	scorer     trending.Scorer[talkgroups.ID]
	scores     trending.Scores[talkgroups.ID]
	lastScore  time.Time
	sim        *Simulation
	alertCache map[talkgroups.ID]alert.Alert
	renotify   time.Duration
	notifier   notify.Notifier
	tgCache    tgstore.Store
}

type offsetClock time.Duration

func (c *offsetClock) Now() time.Time {
	return time.Now().Add(c.Duration())
}

func (c *offsetClock) Duration() time.Duration {
	return time.Duration(*c)
}

// OffsetClock returns a clock whose Now() method returns the specified offset from the current time.
func OffsetClock(d time.Duration) offsetClock {
	return offsetClock(d)
}

type AlertOption func(*alerter)

// WithClock makes the alerter use a simulated clock.
func WithClock(clock timeseries.Clock) AlertOption {
	return func(as *alerter) {
		as.clock = clock
	}
}

// WithNotifier sets the notifier
func WithNotifier(n notify.Notifier) AlertOption {
	return func(as *alerter) {
		as.notifier = n
	}
}

// New creates a new Alerter using the provided configuration.
func New(cfg config.Alerting, tgCache tgstore.Store, opts ...AlertOption) Alerter {
	if !cfg.Enable {
		return &noopAlerter{}
	}

	as := &alerter{
		cfg:        cfg,
		alertCache: make(map[talkgroups.ID]alert.Alert),
		clock:      timeseries.DefaultClock,
		renotify:   DefaultRenotify,
		tgCache:    tgCache,
	}

	as.reload()

	for _, opt := range opts {
		opt(as)
	}

	as.scorer = trending.NewScorer(
		trending.WithTimeSeries(as.newTimeSeries),
		trending.WithStorageDuration[talkgroups.ID](time.Hour*24*time.Duration(cfg.LookbackDays)),
		trending.WithRecentDuration[talkgroups.ID](time.Duration(cfg.Recent)),
		trending.WithHalfLife[talkgroups.ID](time.Duration(cfg.HalfLife)),
		trending.WithScoreThreshold[talkgroups.ID](ScoreThreshold),
		trending.WithCountThreshold[talkgroups.ID](CountThreshold),
		trending.WithClock[talkgroups.ID](as.clock),
	)

	return as
}

func (as *alerter) reload() {
	if as.cfg.Renotify != nil {
		as.renotify = as.cfg.Renotify.Duration()
	}
}

func (as *alerter) HUP(cfg *config.Config) {
	as.Lock()
	defer as.Unlock()

	log.Debug().Msg("reloading alert config")
	as.cfg = cfg.Alerting
	as.reload()
}

// Go is the alerting loop. It does not start a goroutine.
func (as *alerter) Go(ctx context.Context) {
	ctx = entities.CtxWithServiceSubject(ctx, "alerter")

	err := as.startBackfill(ctx)
	if err != nil {
		log.Error().Err(err).Msg("backfill")
	}

	as.score(time.Now())
	ticker := time.NewTicker(alerterTickInterval)

	for {
		select {
		case now := <-ticker.C:
			as.score(now)
			err := as.notify(ctx)
			if err != nil {
				log.Error().Err(err).Msg("notify")
			}
			as.cleanCache()
		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}

}

func (as *alerter) eval(ctx context.Context, now time.Time, testMode bool) ([]alert.Alert, error) {
	err := as.tgCache.Hint(ctx, as.scoredTGs())
	if err != nil {
		return nil, fmt.Errorf("prime TG cache: %w", err)
	}

	as.Lock()
	defer as.Unlock()

	db := database.FromCtx(ctx)

	var notifications []alert.Alert
	for _, s := range as.scores {
		origScore := s.Score
		tgr, err := as.tgCache.TG(ctx, s.ID)
		if err != nil {
			log.Error().Err(err).Msg("alerting eval tg get")
			continue
		}

		if !tgr.Talkgroup.Alert {
			continue
		}

		if s.Score > as.cfg.AlertThreshold || testMode {
			if old, inCache := as.alertCache[s.ID]; !inCache || now.Sub(old.Timestamp) > as.renotify {
				s.Score *= as.tgCache.Weight(ctx, s.ID, now)
				a, err := alert.Make(ctx, s, origScore)
				if err != nil {
					return nil, fmt.Errorf("makeAlert: %w", err)
				}

				if s.Score < as.cfg.AlertThreshold {
					a.Suppressed = true
				}

				as.alertCache[s.ID] = a

				if !testMode {
					err = db.AddAlert(ctx, a.ToAddAlertParams())
					if err != nil {
						return nil, fmt.Errorf("addAlert: %w", err)
					}
				}

				if !a.Suppressed {
					notifications = append(notifications, a)
				}
			}
		}
	}

	return notifications, nil

}

func (as *alerter) testNotifyHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ridx := rand.Intn(len(as.scores))
	a, err := alert.Make(ctx, as.scores[ridx], 1.0)
	if err != nil {
		log.Error().Err(err).Msg("test notify make alert fail")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	alerts, err := as.eval(ctx, time.Now(), true)
	if err != nil {
		log.Error().Err(err).Msg("test notification eval")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	alerts = append(alerts, a)

	err = as.notifier.Send(ctx, alerts)
	if err != nil {
		log.Error().Err(err).Msg("test notification send")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, _ = w.Write([]byte("Sent"))
}

// scoredTGs gets a list of TGs.
func (as *alerter) scoredTGs() []talkgroups.ID {
	tgs := make([]talkgroups.ID, 0, len(as.scores))
	for _, s := range as.scores {
		tgs = append(tgs, s.ID)
	}

	return tgs
}

// packedScoredTGs gets a list of TGID tuples.
func (as *alerter) scoredTGsTuple() (tgs database.TGTuples) {
	tgs = database.MakeTGTuples(len(as.scores))
	for _, s := range as.scores {
		tgs.Append(s.ID.System, s.ID.Talkgroup)
	}

	return tgs
}

// notify iterates the scores and sends out any necessary notifications
func (as *alerter) notify(ctx context.Context) error {
	if as.notifier == nil {
		return nil
	}

	notifications, err := as.eval(ctx, time.Now(), false)
	if err != nil {
		return err
	}

	if len(notifications) > 0 {
		return as.notifier.Send(ctx, notifications)
	}

	return nil
}

// cleanCache clears the cache of aged-out entries
func (as *alerter) cleanCache() {
	if as.notifier == nil {
		return
	}

	now := time.Now()

	as.Lock()
	defer as.Unlock()

	for k, a := range as.alertCache {
		if now.Sub(a.Timestamp) > as.renotify {
			delete(as.alertCache, k)
		}
	}
}

func (as *alerter) newTimeSeries(id talkgroups.ID) trending.TimeSeries {
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

func (as *alerter) startBackfill(ctx context.Context) error {
	now := time.Now()
	since := now.Add(-24 * time.Hour * time.Duration(as.cfg.LookbackDays))
	log.Debug().Time("since", since).Msg("starting stats backfill")
	count, err := as.backfill(ctx, since, now)
	if err != nil {
		return err
	}
	log.Debug().Int("callsCount", count).Str("in", time.Since(now).String()).Int("tgCount", as.scorer.Score().Len()).Msg("backfill finished")

	return nil
}

func (as *alerter) score(now time.Time) {
	as.Lock()
	defer as.Unlock()

	as.scores = as.scorer.Score()
	as.lastScore = now
	sort.Sort(as.scores)
}

func (as *alerter) backfill(ctx context.Context, since time.Time, until time.Time) (count int, err error) {
	db := database.FromCtx(ctx)
	const backfillStatsQuery = `SELECT system, talkgroup, call_date FROM calls WHERE call_date > $1 AND call_date < $2 ORDER BY call_date ASC`

	rows, err := db.DB().Query(ctx, backfillStatsQuery, since, until)
	if err != nil {
		return count, err
	}
	defer rows.Close()

	as.Lock()
	defer as.Unlock()

	for rows.Next() {
		var tg talkgroups.ID
		var callDate time.Time
		if err := rows.Scan(&tg.System, &tg.Talkgroup, &callDate); err != nil {
			return count, err
		}
		as.scorer.AddEvent(tg, callDate)
		if as.sim != nil { // step the simulator if it is active
			as.sim.stepClock(callDate)
		}
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

func (as *alerter) Call(ctx context.Context, call *calls.Call) error {
	as.Lock()
	defer as.Unlock()
	as.scorer.AddEvent(call.TalkgroupTuple(), call.DateTime)

	return nil
}

func (*alerter) Enabled() bool { return true }

// noopAlerter is used when alerting is disabled.
type noopAlerter struct{}

func (*noopAlerter) SinkType() string                            { return "noopAlerter" }
func (*noopAlerter) Call(_ context.Context, _ *calls.Call) error { return nil }
func (*noopAlerter) Go(_ context.Context)                        {}
func (*noopAlerter) Enabled() bool                               { return false }
func (*noopAlerter) HUP(_ *config.Config)                        {}
