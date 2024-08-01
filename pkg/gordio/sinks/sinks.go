package sinks

import (
	"context"
	"dynatron.me/x/stillbox/pkg/gordio/calls"

	"github.com/rs/zerolog/log"
)

type Sink interface {
	Call(context.Context, *calls.Call) error
	SinkType() string
}

type sinkInstance struct {
	Sink
	Name string
}

type Sinks []sinkInstance

func (s *Sinks) Register(name string, toAdd Sink) {
	*s = append(*s, sinkInstance{
		Name: name,
		Sink: toAdd,
	})
}

func (s *Sinks) EmitCall(ctx context.Context, call *calls.Call) {
	for _, sink := range *s {
		go sink.emitCallLogErr(ctx, call)
	}
}

func (sink *sinkInstance) emitCallLogErr(ctx context.Context, call *calls.Call) {
	err := sink.Call(ctx, call)
	if err != nil {
		log.Error().Str("sink", sink.Name).Err(err).Msg("call emit to sink failed")
	}
}
