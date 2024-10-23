package sinks

import (
	"dynatron.me/x/stillbox/pkg/gordio/alerting"
)

func NewAlerterSink(a *alerting.Alerter) *alerting.Alerter {
	return a
}
