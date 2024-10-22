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

type Sinks struct {
	sync.RWMutex
	sinks []sinkInstance
}

func (s *Sinks) Register(name string, toAdd Sink, required bool) {
	s.Lock()
	defer s.Unlock()

	s.sinks = append(s.sinks, sinkInstance{
		Name:     name,
		Sink:     toAdd,
		Required: required,
	})
}

func (s *Sinks) EmitCall(ctx context.Context, call *calls.Call) error {
	s.Lock()
	defer s.Unlock()

	g, ctx := errgroup.WithContext(ctx)
	for i := range s.sinks {
		sink := s.sinks[i]
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
