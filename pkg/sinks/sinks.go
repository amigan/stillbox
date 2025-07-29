package sinks

import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"

	"dynatron.me/x/stillbox/internal/common"
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
	Flags Flags
}

type Sinks interface {
	Register(toAdd Sink, flags ...Flags)
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

func (s *sinks) Register(toAdd Sink, flags ...Flags) {
	s.Lock()
	defer s.Unlock()

	var f Flags
	for _, ft := range flags {
		f |= ft
	}

	s.sinks[toAdd] = sinkInstance{
		Sink:  toAdd,
		Flags: f,
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
		if sink.Flags.Has(AsyncFlag) {
			go sink.callEmitter(ctx, call)
		} else {
			g.Go(sink.callEmitter(ctx, call))
		}
	}

	return g.Wait()
}

type Flags int

func (f Flags) Has(h Flags) bool {
	return f&h != 0
}

const (
	NoneFlag Flags = 0

	RequiredFlag Flags = 1 << iota
	AsyncFlag
)

func (sink *sinkInstance) callEmitter(ctx context.Context, call *calls.Call) func() error {
	return func() (err error) {
		defer func() { // for errgroup
			if rec := recover(); rec != nil {
				err = common.FromPanicValue(rec)
			}
		}()
		err = sink.Call(ctx, call)
		if err != nil {
			if sink.Flags.Has(RequiredFlag) {
				return err
			} else {
				log.Error().Str("sink", sink.Name()).Err(err).Msg("call emit to sink failed")
			}
		}

		return nil
	}
}
