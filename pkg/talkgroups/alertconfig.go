package talkgroups

import (
	"encoding/json"
	"sync"
	"time"

	"dynatron.me/x/stillbox/internal/ruletime"
	"dynatron.me/x/stillbox/internal/trending"
)

type AlertConfig struct {
	sync.RWMutex
	m map[ID][]AlertRule
}

type AlertRule struct {
	Times           []ruletime.RuleTime `json:"times"`
	ScoreMultiplier float32             `json:"mult"`
}

func NewAlertConfig() AlertConfig {
	return AlertConfig{
		m: make(map[ID][]AlertRule),
	}
}

func (ac *AlertConfig) GetRules(tg ID) []AlertRule {
	ac.RLock()
	defer ac.RUnlock()

	return ac.m[tg]
}

func (ac *AlertConfig) UnmarshalTGRules(tg ID, confBytes []byte) error {
	ac.Lock()
	defer ac.Unlock()

	if len(confBytes) == 0 {
		return nil
	}

	var rules []AlertRule
	err := json.Unmarshal(confBytes, &rules)
	if err != nil {
		return err
	}

	ac.m[tg] = rules
	return nil
}

func (ac *AlertConfig) ApplyAlertRules(score trending.Score[ID], t time.Time, coversOpts ...ruletime.CoversOption) float64 {
	ac.RLock()
	s, has := ac.m[score.ID]
	ac.RUnlock()
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

func (ac *AlertConfig) Invalidate() {
	ac.Lock()
	defer ac.Unlock()

	clear(ac.m)
}

func (ar *AlertRule) MatchTime(t time.Time, coversOpts ...ruletime.CoversOption) bool {
	for _, at := range ar.Times {
		if at.Covers(t, coversOpts...) {
			return true
		}
	}

	return false
}
