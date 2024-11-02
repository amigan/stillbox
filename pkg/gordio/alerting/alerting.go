package alerting

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"text/template"
	"time"

	cl "dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/gordio/config"
	"dynatron.me/x/stillbox/pkg/gordio/database"
	"dynatron.me/x/stillbox/pkg/gordio/notify"
	"dynatron.me/x/stillbox/pkg/gordio/sinks"

	"dynatron.me/x/stillbox/internal/timeseries"
	"dynatron.me/x/stillbox/internal/trending"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
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

	stats
}

type alerter struct {
	sync.RWMutex
	clock      timeseries.Clock
	cfg        config.Alerting
	scorer     trending.Scorer[cl.Talkgroup]
	scores     trending.Scores[cl.Talkgroup]
	lastScore  time.Time
	sim        *Simulation
	alertCache map[cl.Talkgroup]Alert
	renotify   time.Duration
	notifier   notify.Notifier
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
func New(cfg config.Alerting, opts ...AlertOption) Alerter {
	if !cfg.Enable {
		return &noopAlerter{}
	}

	as := &alerter{
		cfg:        cfg,
		alertCache: make(map[cl.Talkgroup]Alert),
		clock:      timeseries.DefaultClock,
		renotify:   DefaultRenotify,
	}

	if cfg.Renotify != nil {
		as.renotify = cfg.Renotify.Duration()
	}

	for _, opt := range opts {
		opt(as)
	}

	as.scorer = trending.NewScorer(
		trending.WithTimeSeries(as.newTimeSeries),
		trending.WithStorageDuration[cl.Talkgroup](time.Hour*24*time.Duration(cfg.LookbackDays)),
		trending.WithRecentDuration[cl.Talkgroup](time.Duration(cfg.Recent)),
		trending.WithHalfLife[cl.Talkgroup](time.Duration(cfg.HalfLife)),
		trending.WithScoreThreshold[cl.Talkgroup](ScoreThreshold),
		trending.WithCountThreshold[cl.Talkgroup](CountThreshold),
		trending.WithClock[cl.Talkgroup](as.clock),
	)

	return as
}

// Go is the alerting loop. It does not start a goroutine.
func (as *alerter) Go(ctx context.Context) {
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

const notificationTemplStr = `{{ range . -}}
{{ .TGName }} is active with a score of {{ f .Score.Score 4 }}! ({{ f .Score.RecentCount 0 }}/{{ .Score.Count }} recent calls)

{{ end -}}`

var notificationTemplate = template.Must(template.New("notification").Funcs(funcMap).Parse(notificationTemplStr))

func (as *alerter) eval(ctx context.Context, now time.Time, add bool) ([]Alert, error) {
	tgc, err := cl.NewTalkgroupCache(ctx, as.packedScoredTGs())
	if err != nil {
		return nil, fmt.Errorf("new TG cache: %w", err)
	}

	db := database.FromCtx(ctx)

	var notifications []Alert
	for _, s := range as.scores {
		tgr, has := tgc.TG(s.ID)
		if has {
			if !tgr.Alert {
				continue
			}
			s.Score *= float64(tgr.Weight)
		}
		origScore := s.Score
		s.Score = tgc.ScaleScore(s, now)

		if s.Score > as.cfg.AlertThreshold {
			if old, inCache := as.alertCache[s.ID]; !inCache || now.Sub(old.Timestamp) > as.renotify {
				a, err := makeAlert(tgc, s, origScore)
				if err != nil {
					return nil, fmt.Errorf("makeAlert: %w", err)
				}

				as.alertCache[s.ID] = a

				if add {
					err = db.AddAlert(ctx, a.ToAddAlertParams())
					if err != nil {
						return nil, fmt.Errorf("addAlert: %w", err)
					}
				}

				notifications = append(notifications, a)
			}
		}
	}

	return notifications, nil 

}

func (as *alerter) testNotifyHandler(w http.ResponseWriter, r *http.Request) {
	as.RLock()
	defer as.RUnlock()
	alerts := make([]Alert, 0, len(as.scores))
	ctx := r.Context()

	alerts, err := as.eval(ctx, time.Now(), false)
	if err != nil {
		log.Error().Err(err).Msg("test notification send")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}


	err = as.sendNotification(ctx, alerts)
	if err != nil {
		log.Error().Err(err).Msg("test notification send")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, _ = w.Write([]byte("Sent"))
}

// packedScoredTGs gets a packed list of TG IDs for DB use.
func (as *alerter) packedScoredTGs() []int64 {
	packedTGs := make([]int64, 0, len(as.scores))
	for _, s := range as.scores {
		packedTGs = append(packedTGs, s.ID.Pack())
	}

	return packedTGs
}

// notify iterates the scores and sends out any necessary notifications
func (as *alerter) notify(ctx context.Context) error {
	if as.notifier == nil {
		return nil
	}

	as.Lock()
	defer as.Unlock()

	notifications, err := as.eval(ctx, time.Now(), true)
	if err != nil {
		return err
	}

	if len(notifications) > 0 {
		return as.sendNotification(ctx, notifications)
	}

	return nil
}

type Alert struct {
	ID        uuid.UUID
	Timestamp time.Time
	TGName    string
	Score     trending.Score[cl.Talkgroup]
	OrigScore float64
	Weight    float32
}

func (a *Alert) ToAddAlertParams() database.AddAlertParams {
	f32score := float32(a.Score.Score)
	f32origscore := float32(a.OrigScore)
	return database.AddAlertParams{
		ID:       a.ID,
		Time:     pgtype.Timestamptz{Time: a.Timestamp, Valid: true},
		PackedTg: a.Score.ID.Pack(),
		Weight:   &a.Weight,
		Score:    &f32score,
		OrigScore: &f32origscore,
	}
}

// sendNotification renders and sends the notification.
func (as *alerter) sendNotification(ctx context.Context, n []Alert) error {
	msgBuffer := new(bytes.Buffer)

	err := notificationTemplate.Execute(msgBuffer, n)
	if err != nil {
		return fmt.Errorf("notification template render: %w", err)
	}

	log.Debug().Str("msg", msgBuffer.String()).Msg("notifying")

	return as.notifier.Send(ctx, NotificationSubject, msgBuffer.String())
}

// makeAlert creates a notification for later rendering by the template.
// It takes a talkgroup Score as input.
func makeAlert(tgs *cl.TalkgroupCache, score trending.Score[cl.Talkgroup], origScore float64) (Alert, error) {
	d := Alert{
		ID:        uuid.New(),
		Score:     score,
		Timestamp: time.Now(),
		Weight:    1.0,
		OrigScore: origScore,
	}

	tgRecord, has := tgs.TG(score.ID)
	switch has {
	case true:
		d.Weight = tgRecord.Weight
		if tgRecord.SystemName == "" {
			tgRecord.SystemName = strconv.Itoa(int(score.ID.System))
		}

		if tgRecord.Name != nil {
			d.TGName = fmt.Sprintf("%s %s (%d)", tgRecord.SystemName, *tgRecord.Name, score.ID.Talkgroup)
		} else {
			d.TGName = fmt.Sprintf("%s:%d", tgRecord.SystemName, int(score.ID.Talkgroup))
		}
	case false:
		system, has := tgs.SystemName(int(score.ID.System))
		if has {
			d.TGName = fmt.Sprintf("%s:%d", system, int(score.ID.Talkgroup))
		} else {
			d.TGName = fmt.Sprintf("%d:%d", int(score.ID.System), int(score.ID.Talkgroup))
		}
	}

	return d, nil
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

	rows, err := db.Query(ctx, backfillStatsQuery, since, until)
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

func (as *alerter) Call(ctx context.Context, call *cl.Call) error {
	as.Lock()
	defer as.Unlock()
	as.scorer.AddEvent(call.TalkgroupTuple(), call.DateTime)

	return nil
}

func (*alerter) Enabled() bool { return true }

// noopAlerter is used when alerting is disabled.
type noopAlerter struct{}

func (*noopAlerter) SinkType() string                         { return "noopAlerter" }
func (*noopAlerter) Call(_ context.Context, _ *cl.Call) error { return nil }
func (*noopAlerter) Go(_ context.Context)                     {}
func (*noopAlerter) Enabled() bool                            { return false }
