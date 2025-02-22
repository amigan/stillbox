package stats

import (
	"context"
	"time"

	"dynatron.me/x/stillbox/internal/cache"
	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/calls/callstore"
	"dynatron.me/x/stillbox/pkg/services"
)

const DefaultExpiration = 5 * time.Minute

type Stats interface {
	GetCallStats(ctx context.Context, interval calls.StatsInterval) (*calls.Stats, error)
}

type statsKeyType string

const StatsKey statsKeyType = "stats"

func CtxWithStats(ctx context.Context, s Stats) context.Context {
	return services.WithValue(ctx, StatsKey, s)
}

func FromCtx(ctx context.Context) Stats {
	s, ok := services.Value(ctx, StatsKey).(Stats)
	if !ok {
		panic("no stats in context")
	}

	return s
}

type stats struct {
	cs callstore.Store
	sc cache.Cache[calls.StatsInterval, *calls.Stats]
}

func NewStats(cst callstore.Store, cacheExp time.Duration) *stats {
	s := &stats{
		cs: cst,
		sc: cache.New[calls.StatsInterval, *calls.Stats](cache.WithExpiration(cacheExp)),
	}

	return s
}

func (s *stats) GetCallStats(ctx context.Context, interval calls.StatsInterval) (*calls.Stats, error) {
	st, has := s.sc.Get(interval)
	if has {
		return st, nil
	}

	var start time.Time
	now := time.Now()
	end := now
	bnd := common.NewTimeBounder(common.WithLocation(now.Location()))

	switch interval {
	case calls.IntervalHour:
		start = now.Add(-24 * time.Hour) // one day
	case calls.IntervalDay:
		start = now.Add(-7 * 24 * time.Hour) // one week
	case calls.IntervalWeek:
		start, end = bnd.GetMonthlyBounds(now)
		start, _ = bnd.GetWeeklyBounds(start)
		_, end = bnd.GetWeeklyBounds(end)
	case calls.IntervalMonth:
		start = now.Add(-365 * 24 * time.Hour) // one year
	default:
		return nil, calls.ErrInvalidInterval
	}

	st, err := s.cs.CallStats(ctx, interval, jsontypes.Time(start), jsontypes.Time(end))
	if err != nil {
		return nil, err
	}

	s.sc.Set(interval, st)

	return st, nil
}
