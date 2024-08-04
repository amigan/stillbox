package sinks

import (
	"context"

	"dynatron.me/x/stillbox/pkg/gordio/calls"
	"dynatron.me/x/stillbox/pkg/gordio/nexus"
)

type NexusSink struct {
	nexus *nexus.Nexus
}

func NewNexusSink(nexus *nexus.Nexus) *NexusSink {
	ns := &NexusSink{
		nexus: nexus,
	}

	return ns
}

func (ns *NexusSink) SinkType() string {
	return "nexus"
}

func (ns *NexusSink) Call(ctx context.Context, call *calls.Call) error {
	return nil
}
