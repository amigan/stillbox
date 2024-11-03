package talkgroups

import (
	"encoding/json"
	"time"

	"dynatron.me/x/stillbox/internal/ruletime"
	"dynatron.me/x/stillbox/internal/trending"
)

type AlertConfig map[ID][]AlertRule

type AlertRule struct {
	Times           []ruletime.RuleTime `json:"times"`
	ScoreMultiplier float32             `json:"mult"`
}

func (ac AlertConfig) AddAlertConfig(tg ID, confBytes []byte) error {
	if len(confBytes) == 0 {
		return nil
	}

	var rules []AlertRule
	err := json.Unmarshal(confBytes, &rules)
	if err != nil {
		return err
	}

	ac[tg] = rules
	return nil
}

func (ac AlertConfig) ApplyAlertRules(score trending.Score[ID], t time.Time, coversOpts ...ruletime.CoversOption) float64 {
	s, has := ac[score.ID]
	if !has {
		return score.Score
	}

	final := score.Score

	for _, ar := range s {
		if ar.MatchTime(t, coversOpts...) {
			final *= float64(ar.ScoreMultiplier)
		}
	}

	return final
}

func (ar *AlertRule) MatchTime(t time.Time, coversOpts ...ruletime.CoversOption) bool {
	for _, at := range ar.Times {
		if at.Covers(t, coversOpts...) {
			return true
		}
	}

	return false
}
