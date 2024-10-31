package alerting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"dynatron.me/x/stillbox/internal/forms"
	"dynatron.me/x/stillbox/internal/jsontime"
	"dynatron.me/x/stillbox/internal/trending"
	cl "dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/gordio/config"

	"github.com/rs/zerolog/log"
)

// A Simulation simulates what happens to the alerter during a specified time period using past data from the database.
type Simulation struct {
	// normal Alerting config
	config.Alerting

	// ScoreStart is the time when scoring begins
	ScoreStart jsontime.Time `json:"scoreStart" yaml:"scoreStart" form:"scoreStart"`
	// ScoreEnd is the time when the score simulator ends. Left blank, it defaults to time.Now()
	ScoreEnd jsontime.Time `json:"scoreEnd" yaml:"scoreEnd" form:"scoreEnd"`

	// SimInterval is the interval at which the scorer will be called
	SimInterval jsontime.Duration `json:"simInterval" yaml:"simInterval" form:"simInterval"`

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
func (s *Simulation) Simulate(ctx context.Context) trending.Scores[cl.Talkgroup] {
	now := time.Now()
	s.Enable = true
	s.alerter = New(s.Alerting, WithClock(&s.clock)).(*alerter)
	if time.Time(s.ScoreEnd).IsZero() {
		s.ScoreEnd = jsontime.Time(now)
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
	s.backfill(ctx, sinceLookback, time.Time(s.ScoreStart))

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
	s.backfill(ctx, time.Time(s.ScoreStart), scoreEnd)

	s.lastScore = scoreEnd
	sort.Sort(s.scores)

	return s.scores
}

// simulateHandler is the POST endpoint handler.
func (as *alerter) simulateHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s := new(Simulation)
	switch r.Header.Get("Content-Type") {
	case "application/json":
		err := json.NewDecoder(r.Body).Decode(s)
		if err != nil {
			err = fmt.Errorf("simulate decode: %w", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	default:
		err := forms.Unmarshal(r, s, forms.WithAcceptBlank(), forms.WithParseLocalTime())
		if err != nil {
			err = fmt.Errorf("simulate unmarshal: %w", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	err := s.verify()
	if err != nil {
		err = fmt.Errorf("simulation profile verify: %w", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.Simulate(ctx)
	s.tgStatsHandler(w, r)
}
