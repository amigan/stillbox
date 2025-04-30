package alerting

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"dynatron.me/x/stillbox/internal/forms"
	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/internal/trending"
	"dynatron.me/x/stillbox/pkg/authz"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/metrics"
	"dynatron.me/x/stillbox/pkg/talkgroups"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"

	"github.com/rs/zerolog/log"
)

// A Simulation simulates what happens to the alerter during a specified time period using past data from the database.
type Simulation struct {
	// normal Alerting config
	config.Alerting

	// ScoreStart is the time when scoring begins
	ScoreStart jsontypes.Time `json:"scoreStart" yaml:"scoreStart" form:"scoreStart"`
	// ScoreEnd is the time when the score simulator ends. Left blank, it defaults to time.Now()
	ScoreEnd jsontypes.Time `json:"scoreEnd" yaml:"scoreEnd" form:"scoreEnd"`

	// SimInterval is the interval at which the scorer will be called
	SimInterval jsontypes.Duration `json:"simInterval" yaml:"simInterval" form:"simInterval"`

	clock    offsetClock `json:"-"`
	*alerter `json:"-"`
}

func (s *Simulation) verify() error {
	switch {
	case !s.ScoreEnd.Time().IsZero() && s.ScoreStart.Time().After(s.ScoreEnd.Time()):
		return errors.New("end is before start")
	case s.LookbackDays > 14:
		return errors.New("lookback days >14")
	}
	return nil
}

// stepClock is called by backfill during simulation operations.
func (s *Simulation) stepClock(t time.Time) {
	now := s.clock.Now()
	step := t.Sub(s.lastScore)
	if step > time.Duration(s.SimInterval) {
		s.clock += offsetClock(s.SimInterval)
		s.scores = s.scorer.Score()
		s.lastScore = now
	}

}

// Simulate begins the simulation using the DB handle from ctx. It returns final scores.
func (s *Simulation) Simulate(ctx context.Context) (trending.Scores[talkgroups.ID], error) {
	db := database.FromCtx(ctx)
	now := time.Now()
	tgc := tgstore.NewCache(db, metrics.NewNoOp())

	s.Enable = true
	s.alerter = New(s.Alerting, tgc, WithClock(&s.clock)).(*alerter)
	if time.Time(s.ScoreEnd).IsZero() {
		s.ScoreEnd = jsontypes.Time(now)
	}
	log.Debug().Time("scoreStart", s.ScoreStart.Time()).
		Time("scoreEnd", s.ScoreEnd.Time()).
		Str("interval", s.SimInterval.String()).
		Uint("lookbackDays", s.LookbackDays).
		Msg("simulation start")

	scoreEnd := time.Time(s.ScoreEnd)

	// compute lookback start Time
	sinceLookback := time.Time(scoreEnd).Add(-24 * time.Hour * time.Duration(s.LookbackDays))

	// backfill from lookback start until score start
	_, err := s.backfill(ctx, sinceLookback, time.Time(s.ScoreStart))
	if err != nil {
		return nil, fmt.Errorf("simulate backfill: %w", err)
	}

	// initial score
	s.scores = s.scorer.Score()
	s.lastScore = time.Time(s.ScoreStart)

	ssT := time.Time(s.ScoreStart)
	nowDiff := now.Sub(time.Time(ssT))

	// and set the clock offset to it
	s.clock -= offsetClock(nowDiff)

	// turn on sim mode
	s.alerter.sim = s

	// compute time since score start until now
	// backfill from scorestart until now. sim is enabled, so scoring will be done by stepClock()
	_, err = s.backfill(ctx, time.Time(s.ScoreStart), scoreEnd)
	if err != nil {
		return nil, fmt.Errorf("simulate backfill final: %w", err)
	}

	s.lastScore = scoreEnd
	sort.Sort(s.scores)

	return s.scores, nil
}

// simulateHandler is the POST endpoint handler.
func (as *alerter) simulateHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	_, err := authz.Check(ctx, authz.UseResource(entities.ResourceAlert), authz.WithActions(entities.ActionSimulate))
	if authz.IsErrAccessDenied(err) != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if err != nil {
		log.Error().Err(err).Msg("rbac check failed")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s := new(Simulation)

	err = forms.Unmarshal(r, s, forms.WithAcceptBlank(), forms.WithParseLocalTime())
	if err != nil {
		err = fmt.Errorf("simulate unmarshal: %w", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = s.verify()
	if err != nil {
		err = fmt.Errorf("simulation profile verify: %w", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err = s.Simulate(ctx)
	if err != nil {
		err = fmt.Errorf("simulate: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.tgStatsHandler(w, r)
}
