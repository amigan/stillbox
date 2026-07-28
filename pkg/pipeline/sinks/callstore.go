package sinks

import (
	"context"

	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/calls/callstore"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
)

type CallstoreSink struct {
	store callstore.Store
	tgs   tgstore.Store
}

func NewCallstoreSink(store callstore.Store, tgs tgstore.Store) *CallstoreSink {
	return &CallstoreSink{store, tgs}
}

func (s *CallstoreSink) Name() string {
	return s.SinkType()
}

func (s *CallstoreSink) Call(ctx context.Context, call *calls.Call) error {
	if call.Filtered() {
		return nil
	}

	return s.store.AddCall(ctx, call)
}

func (s *CallstoreSink) SinkType() string {
	return "callstore"
}
