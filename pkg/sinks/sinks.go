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
}

type sinkInstance struct {
	Sink
	Name string

	// whether call ingest should be considered failed if this sink returns error
	Required bool
}

type Sinks interface {
	Register(name string, toAdd Sink, required bool)
	Unregister(name string)
	Shutdown()
	EmitCall(ctx context.Context, call *calls.Call) error
}

type sinks struct {
	sync.RWMutex
	sinks map[string]sinkInstance
}

func NewSinkManager() *sinks {
	return &sinks{
		sinks: make(map[string]sinkInstance),
	}
}

func (s *sinks) Register(name string, toAdd Sink, required bool) {
	s.Lock()
	defer s.Unlock()

	s.sinks[name] = sinkInstance{
		Name:     name,
		Sink:     toAdd,
		Required: required,
	}
}

func (s *sinks) Unregister(name string) {
	s.Lock()
	defer s.Unlock()

	delete(s.sinks, name)
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
			log.Error().Str("sink", sink.Name).Err(err).Msg("call emit to sink failed")
			if sink.Required {
				return err
			}
		}

		return nil
	}
}
