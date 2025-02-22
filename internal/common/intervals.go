package common

import (
	"time"
)

const (
	DaysInWeek      = 7
	MonthsInQuarter = 3
)

type TimeBounder interface {
	GetDailyBounds(date time.Time) (lowerBound, upperBound time.Time)
	GetWeeklyBounds(date time.Time) (lowerBound, upperBound time.Time)
	GetMonthlyBounds(date time.Time) (lowerBound, upperBound time.Time)
	GetQuarterlyBounds(date time.Time) (lowerBound, upperBound time.Time)
	GetYearlyBounds(date time.Time) (lowerBound, upperBound time.Time)
}

type tbOpt func(*timeBounder)

func WithLocation(l *time.Location) tbOpt {
	return func(tb *timeBounder) {
		tb.loc = l
	}
}

func NewTimeBounder(opts ...tbOpt) timeBounder {
	tb := timeBounder{}

	for _, opt := range opts {
		opt(&tb)
	}

	if tb.loc == nil {
		tb.loc = time.UTC
	}

	return tb
}

type timeBounder struct {
	loc *time.Location
}

func (tb timeBounder) GetDailyBounds(date time.Time) (lowerBound, upperBound time.Time) {
	lowerBound = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, tb.loc)
	upperBound = lowerBound.AddDate(0, 0, 1)

	return
}

func (tb timeBounder) GetWeeklyBounds(date time.Time) (lowerBound, upperBound time.Time) {
	lowerBound = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, tb.loc).AddDate(0, 0, -int(date.Weekday()-time.Monday))
	upperBound = lowerBound.AddDate(0, 0, DaysInWeek)

	return
}

func (tb timeBounder) GetMonthlyBounds(date time.Time) (lowerBound, upperBound time.Time) {
	lowerBound = time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, tb.loc)
	upperBound = lowerBound.AddDate(0, 1, 0)

	return
}

func (tb *timeBounder) GetQuarterlyBounds(date time.Time) (lowerBound, upperBound time.Time) {
	year, _, _ := date.Date()

	quarter := (int(date.Month()) - 1) / MonthsInQuarter
	firstMonthOfTheQuarter := time.Month(quarter*MonthsInQuarter + 1)

	lowerBound = time.Date(year, firstMonthOfTheQuarter, 1, 0, 0, 0, 0, tb.loc)
	upperBound = lowerBound.AddDate(0, MonthsInQuarter, 0)

	return
}

func (tb timeBounder) GetYearlyBounds(date time.Time) (lowerBound, upperBound time.Time) {
	lowerBound = time.Date(date.Year(), 1, 1, 0, 0, 0, 0, tb.loc)
	upperBound = lowerBound.AddDate(1, 0, 0)

	return
}
