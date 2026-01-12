package common

import (
	"fmt"
	"time"
)

const (
	DaysInWeek      = 7
	MonthsInQuarter = 3
)

type tbOpt func(*TimeBounder)

func WithLocation(l *time.Location) tbOpt {
	return func(tb *TimeBounder) {
		tb.loc = l
	}
}

func WithDefaultBounds(i Interval) tbOpt {
	return func(tb *TimeBounder) {
		tb.defaultIntvl = &i
	}
}

func NewTimeBounder(opts ...tbOpt) *TimeBounder {
	tb := &TimeBounder{}

	for _, opt := range opts {
		opt(tb)
	}

	if tb.loc == nil {
		tb.loc = time.UTC
	}

	if tb.defaultIntvl != nil {
		switch *tb.defaultIntvl {
		case Daily:
			tb.defaultBounder = tb.GetDailyBounds
		case Weekly:
			tb.defaultBounder = tb.GetWeeklyBounds
		case Monthly:
			tb.defaultBounder = tb.GetMonthlyBounds
		case Quarterly:
			tb.defaultBounder = tb.GetQuarterlyBounds
		case Yearly:
			tb.defaultBounder = tb.GetYearlyBounds
		default:
			panic("unknown interval")
		}
	}

	return tb
}

func (tb *TimeBounder) Bounds(t time.Time) (lowerBound, upperBound time.Time) {
	return tb.defaultBounder(t)
}

type TimeBounder struct {
	loc            *time.Location
	defaultIntvl   *Interval
	defaultBounder func(time.Time) (time.Time, time.Time)
}

func (tb *TimeBounder) GetDailyBounds(date time.Time) (lowerBound, upperBound time.Time) {
	date = date.In(tb.loc)
	lowerBound = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, tb.loc)
	upperBound = lowerBound.AddDate(0, 0, 1)

	return
}

func (tb *TimeBounder) GetWeeklyBounds(date time.Time) (lowerBound, upperBound time.Time) {
	date = date.In(tb.loc)
	lowerBound = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, tb.loc).AddDate(0, 0, -int(date.Weekday()-time.Monday))
	upperBound = lowerBound.AddDate(0, 0, DaysInWeek)

	return
}

func (tb *TimeBounder) GetMonthlyBounds(date time.Time) (lowerBound, upperBound time.Time) {
	date = date.In(tb.loc)
	lowerBound = time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, tb.loc)
	upperBound = lowerBound.AddDate(0, 1, 0)

	return
}

func (tb *TimeBounder) GetQuarterlyBounds(date time.Time) (lowerBound, upperBound time.Time) {
	date = date.In(tb.loc)
	year, _, _ := date.Date()

	quarter := (int(date.Month()) - 1) / MonthsInQuarter
	firstMonthOfTheQuarter := time.Month(quarter*MonthsInQuarter + 1)

	lowerBound = time.Date(year, firstMonthOfTheQuarter, 1, 0, 0, 0, 0, tb.loc)
	upperBound = lowerBound.AddDate(0, MonthsInQuarter, 0)

	return
}

func (tb *TimeBounder) GetYearlyBounds(date time.Time) (lowerBound, upperBound time.Time) {
	date = date.In(tb.loc)
	lowerBound = time.Date(date.Year(), 1, 1, 0, 0, 0, 0, tb.loc)
	upperBound = lowerBound.AddDate(1, 0, 0)

	return
}

type ErrInvalidInterval string

func (in ErrInvalidInterval) Error() string {
	return fmt.Sprintf("invalid interval '%s'", string(in))
}

type Interval string

const (
	Unknown   Interval = ""
	Daily     Interval = "daily"
	Weekly    Interval = "weekly"
	Monthly   Interval = "monthly"
	Quarterly Interval = "quarterly"
	Yearly    Interval = "yearly"
)

func (p Interval) Duration() time.Duration {
	const day = 24 * time.Hour
	return map[Interval]time.Duration{
		Daily:     day,
		Weekly:    7 * day,
		Monthly:   30 * day,
		Quarterly: (365 / 4) * day,
		Yearly:    365 * day,
	}[p]
}

func (p Interval) IsValid() bool {
	switch p {
	case Daily, Weekly, Monthly, Quarterly, Yearly:
		return true
	}

	return false
}
