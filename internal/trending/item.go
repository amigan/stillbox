package trending

import (
	"math"
	"time"
)

type item[K comparable] struct {
	eventSeries TimeSeries
	maxSeries   SlidingWindow

	max     float64
	maxTime time.Time
	options *options[K]

	// TODO: move outside of item because it's the same for all items
	defaultExpectation float64
	defaultHourlyCount float64
}

func newItem[K comparable](id K, options *options[K]) *item[K] {
	defaultHourlyCount := float64(options.baseCount) * float64(options.storageDuration/time.Hour)
	defaultExpectation := float64(options.baseCount) / float64(time.Hour/options.recentDuration)
	return &item[K]{
		eventSeries: options.creator(id, options.clock),
		maxSeries:   options.slidingWindowCreator(id),
		options:     options,

		defaultExpectation: defaultExpectation,
		defaultHourlyCount: defaultHourlyCount,
	}
}

func (i *item[K]) score() Score[K] {
	recentCount, count := i.computeCounts()
	if recentCount < i.options.countThreshold {
		return Score[K]{}
	}
	if recentCount == count {
		// we see this for the first time so there is no historical data
		// use a sensible default like average/median over all items
		count = recentCount + i.defaultHourlyCount
	}
	probability := recentCount / count

	// order of those two lines is important.
	// if we insert before reading we might just get the same value.
	expectation := i.computeRecentMax()
	i.maxSeries.Insert(probability)

	if expectation == 0.0 {
		expectation = i.defaultExpectation
	}

	klScore := computeKullbackLeibler(probability, expectation)
	if klScore > i.max {
		i.updateMax(klScore)
	}
	i.decayMax()

	mixedScore := 5 * (klScore + i.max)

	return Score[K]{
		Score:       mixedScore,
		Probability: probability,
		Expectation: expectation,
		Maximum:     i.max,
		KLScore:     klScore,
		Count:       count,
		RecentCount: recentCount,
	}
}

func (i *item[K]) computeCounts() (float64, float64) {
	now := i.options.clock.Now()
	totalCount, _ := i.eventSeries.Range(now.Add(-i.options.storageDuration), now)
	count, _ := i.eventSeries.Range(now.Add(-i.options.recentDuration), now)
	return count, totalCount
}

func (i *item[K]) computeRecentMax() float64 {
	return i.maxSeries.Max()
}

func (i *item[K]) decayMax() {
	i.updateMax(i.max * i.computeExponentialDecayMultiplier())
}

func (i *item[K]) updateMax(score float64) {
	i.max = score
	i.maxTime = i.options.clock.Now()
}

func (i *item[K]) computeExponentialDecayMultiplier() float64 {
	return math.Pow(0.5, float64(i.options.clock.Now().Unix()-i.maxTime.Unix())/i.options.halfLife.Seconds())
}

func computeKullbackLeibler(probability float64, expectation float64) float64 {
	if probability == 0.0 {
		return 0.0
	}
	return probability * math.Log(probability/expectation)
}
