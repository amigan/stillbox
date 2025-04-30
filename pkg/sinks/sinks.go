package sinks

import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"

	"dynatron.me/x/stillbox/pkg/calls"

	"github.com/rs/zerolog/log"
)

type Sink interface {
	Call(context.Context, *calls.Call) error
	SinkType() string
	Name() string
}

type sinkInstance struct {
	Sink

	// whether call ingest should be considered failed if this sink returns error
	Required bool
}

type Sinks interface {
	Register(toAdd Sink, required bool)
	Unregister(Sink)
	Shutdown()
	EmitCall(ctx context.Context, call *calls.Call) error
}

type sinks struct {
	sync.RWMutex
	sinks map[Sink]sinkInstance
}

func NewSinkManager() *sinks {
	return &sinks{
		sinks: make(map[Sink]sinkInstance),
	}
}

func (s *sinks) Register(toAdd Sink, required bool) {
	s.Lock()
	defer s.Unlock()

	s.sinks[toAdd] = sinkInstance{
		Sink:     toAdd,
		Required: required,
	}
}

func (s *sinks) Unregister(sink Sink) {
	s.Lock()
	defer s.Unlock()

	delete(s.sinks, sink)
}

func (s *sinks) Shutdown() {
	s.Lock()
	defer s.Unlock()

	clear(s.sinks)
}

func (s *sinks) EmitCall(ctx context.Context, call *calls.Call) error {
	s.Lock()
	defer s.Unlock()

	g, ctx := errgroup.WithContext(ctx)
	for _, sink := range s.sinks {
		g.Go(sink.callEmitter(ctx, call))
	}

	return g.Wait()
}

func (sink *sinkInstance) callEmitter(ctx context.Context, call *calls.Call) func() error {
	return func() error {
		err := sink.Call(ctx, call)
		if err != nil {
			if sink.Required {
				return err
			} else {
				log.Error().Str("sink", sink.Name()).Err(err).Msg("call emit to sink failed")
			}
		}

		return nil
	}
}
