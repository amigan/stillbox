package rules

import (
	"time"

	"dynatron.me/x/stillbox/internal/ruletime"
)

type AlertRules []AlertRule

func (ars AlertRules) Apply(t time.Time, coversOpts ...ruletime.CoversOption) float64 {
	if len(ars) < 1 {
		return 1.0
	}

	final := 1.0

	for _, ar := range ars {
		if ar.MatchTime(t, coversOpts...) {
			final *= float64(ar.ScoreMultiplier)
		}
	}

	return final
}

type AlertRule struct {
	Times           []ruletime.RuleTime `json:"times"`
	ScoreMultiplier float32             `json:"mult"`
}

func (ar *AlertRule) MatchTime(t time.Time, coversOpts ...ruletime.CoversOption) bool {
	for _, at := range ar.Times {
		if at.Covers(t, coversOpts...) {
			return true
		}
	}

	return false
}
