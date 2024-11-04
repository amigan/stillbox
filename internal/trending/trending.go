package trending

import (
	"sort"
	"time"

	timeseries "dynatron.me/x/stillbox/internal/timeseries"
	"dynatron.me/x/stillbox/internal/trending/slidingwindow"
)

// Algorithm:
// 1. Divide one week into 5 minutes bins
// The algorithm uses expected probability to compute its ranking.
// By choosing a one week span to compute the expectation the algorithm will forget old trends.
// 2. For every play event increase the counter in the current bin
// 3. Compute the KL Divergence with the following steps
//    - Compute the probability of the last full bin (this should be the current 5 minutes sliding window)
//    - Compute the expected probability over the past bins including the current bin
//    - Compute KL Divergence (kld = p * ln(p/e))
// 4. Keep the highest KL Divergence score together with its timestamp
// 5. Compute exponential decay multiplier and multiply with highest KL Divergence
// 6. Blend current KL Divergence score with decayed high score

var defaultHalfLife = 2 * time.Hour
var defaultRecentDuration = 5 * time.Minute
var defaultStorageDuration = 7 * 24 * time.Hour
var defaultMaxResults = 100
var defaultBaseCount = 1
var defaultScoreThreshold = 0.01
var defaultCountThreshold = 3.0

type options[K comparable] struct {
	creator              TimeSeriesCreator[K]
	slidingWindowCreator SlidingWindowCreator[K]
	clock                timeseries.Clock

	halfLife time.Duration

	recentDuration  time.Duration
	storageDuration time.Duration

	maxResults     int
	baseCount      int
	scoreThreshold float64
	countThreshold float64
}

type Option[K comparable] func(*options[K])

func WithTimeSeries[K comparable](creator TimeSeriesCreator[K]) Option[K] {
	return func(o *options[K]) {
		o.creator = creator
	}
}

func WithSlidingWindow[K comparable](creator SlidingWindowCreator[K]) Option[K] {
	return func(o *options[K]) {
		o.slidingWindowCreator = creator
	}
}

func WithHalfLife[K comparable](halfLife time.Duration) Option[K] {
	return func(o *options[K]) {
		o.halfLife = halfLife
	}
}

func WithRecentDuration[K comparable](recentDuration time.Duration) Option[K] {
	return func(o *options[K]) {
		o.recentDuration = recentDuration
	}
}

func WithStorageDuration[K comparable](storageDuration time.Duration) Option[K] {
	return func(o *options[K]) {
		o.storageDuration = storageDuration
	}
}

func WithMaxResults[K comparable](maxResults int) Option[K] {
	return func(o *options[K]) {
		o.maxResults = maxResults
	}
}

func WithScoreThreshold[K comparable](threshold float64) Option[K] {
	return func(o *options[K]) {
		o.scoreThreshold = threshold
	}
}

func WithCountThreshold[K comparable](threshold float64) Option[K] {
	return func(o *options[K]) {
		o.countThreshold = threshold
	}
}

func WithClock[K comparable](clock timeseries.Clock) Option[K] {
	return func(o *options[K]) {
		o.clock = clock
	}
}

type Scorer[K comparable] struct {
	options options[K]
	items   map[K]*item[K]
}

type SlidingWindow interface {
	Insert(score float64)
	Max() float64
}

type SlidingWindowCreator[K comparable] func(K) SlidingWindow

type TimeSeries interface {
	IncreaseAtTime(amount int, time time.Time)
	Range(start, end time.Time) (float64, error)
}

type TimeSeriesCreator[K comparable] func(K) TimeSeries

func NewMemoryTimeSeries[K comparable](id K) TimeSeries {
	ts, _ := timeseries.NewTimeSeries(timeseries.WithGranularities(
		[]timeseries.Granularity{
			{Granularity: time.Second, Count: 60},
			{Granularity: time.Minute, Count: 10},
			{Granularity: time.Hour, Count: 24},
			{Granularity: time.Hour * 24, Count: 7},
		},
	))
	return ts
}

func NewScorer[K comparable](options ...Option[K]) Scorer[K] {
	scorer := Scorer[K]{items: make(map[K]*item[K])}
	for _, o := range options {
		o(&scorer.options)
	}
	if scorer.options.creator == nil {
		scorer.options.creator = NewMemoryTimeSeries[K]
	}
	if scorer.options.halfLife == 0 {
		scorer.options.halfLife = defaultHalfLife
	}
	if scorer.options.recentDuration == 0 {
		scorer.options.recentDuration = defaultRecentDuration
	}
	if scorer.options.storageDuration == 0 {
		scorer.options.storageDuration = defaultStorageDuration
	}
	if scorer.options.maxResults == 0 {
		scorer.options.maxResults = defaultMaxResults
	}
	if scorer.options.scoreThreshold == 0.0 {
		scorer.options.scoreThreshold = defaultScoreThreshold
	}
	if scorer.options.countThreshold == 0.0 {
		scorer.options.countThreshold = defaultCountThreshold
	}
	if scorer.options.baseCount == 0.0 {
		scorer.options.baseCount = defaultBaseCount
	}
	if scorer.options.clock == nil {
		scorer.options.clock = timeseries.DefaultClock
	}
	if scorer.options.slidingWindowCreator == nil {
		scorer.options.slidingWindowCreator = func(id K) SlidingWindow {
			return slidingwindow.NewSlidingWindow(
				slidingwindow.WithStep(time.Hour*24),
				slidingwindow.WithDuration(scorer.options.storageDuration),
				slidingwindow.WithClock(scorer.options.clock),
			)
		}
	}
	return scorer
}

func (s *Scorer[K]) AddEvent(id K, time time.Time) {
	item := s.items[id]
	if item == nil {
		item = newItem(id, &s.options)
		s.items[id] = item
	}
	s.addToItem(item, time)
}

func (s *Scorer[K]) addToItem(item *item[K], tm time.Time) {
	item.eventSeries.IncreaseAtTime(1, tm)
}

func (s *Scorer[K]) Score() Scores[K] {
	var scores Scores[K]
	for id, item := range s.items {
		score := item.score(id)
		score.ID = id
		scores = append(scores, score)
	}
	sort.Sort(scores)
	if s.options.scoreThreshold > 0 {
		scores = scores.threshold(s.options.scoreThreshold)
	}
	return scores.take(s.options.maxResults)
}
