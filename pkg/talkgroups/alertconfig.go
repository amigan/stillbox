package talkgroups

import (
	"sync"
	"time"

	"dynatron.me/x/stillbox/internal/ruletime"
	"dynatron.me/x/stillbox/pkg/alerting/rules"
)

type AlertConfig struct {
	sync.RWMutex
	m map[ID]rules.AlertRules
}

func NewAlertConfig() AlertConfig {
	return AlertConfig{
		m: make(map[ID]rules.AlertRules),
	}
}

func (ac *AlertConfig) Add(tg ID, r rules.AlertRules) error {
	ac.Lock()
	defer ac.Unlock()

	ac.m[tg] = r
	return nil
}

func (ac *AlertConfig) GetRules(tg ID) rules.AlertRules {
	ac.RLock()
	defer ac.RUnlock()

	return ac.m[tg]
}

func (ac *AlertConfig) ApplyAlertRules(id ID, t time.Time, coversOpts ...ruletime.CoversOption) float64 {
	ac.RLock()
	s, has := ac.m[id]
	ac.RUnlock()
	if !has {
		return 1.0
	}

	return s.Apply(t, coversOpts...)
}

func (ac *AlertConfig) Invalidate() {
	ac.Lock()
	defer ac.Unlock()

	clear(ac.m)
}
