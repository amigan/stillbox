package calls

import (
	"errors"

	"dynatron.me/x/stillbox/internal/jsontypes"
)

type Stats struct {
	Stats    []Stat        `json:"stats"`
	Interval StatsInterval `json:"interval"`
}

type Stat struct {
	Count int64          `json:"count"`
	Time  jsontypes.Time `json:"time"`
}

var (
	ErrInvalidInterval = errors.New("invalid interval")
)

func (s *Stats) GetResourceName() string {
	return "CallStats"
}

type StatsInterval string

const (
	IntervalMinute  StatsInterval = "minute"
	IntervalHour    StatsInterval = "hour"
	IntervalDay     StatsInterval = "day"
	IntervalWeek    StatsInterval = "week"
	IntervalMonth   StatsInterval = "month"
	IntervalQuarter StatsInterval = "quarter"
	IntervalYear    StatsInterval = "year"
)

func (si StatsInterval) IsValid() bool {
	switch si {
	case IntervalMinute, IntervalHour, IntervalDay, IntervalWeek, IntervalMonth, IntervalQuarter, IntervalYear:
		return true
	}

	return false
}
