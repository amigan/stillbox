// Package workers implements transcript (and possibly other) workers.
package workers

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/robin"
	"dynatron.me/x/stillbox/pkg/calls/callstore"
	"dynatron.me/x/stillbox/pkg/calls/filter"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/metrics"
	"dynatron.me/x/stillbox/pkg/nexus/broadcast"
	nxerrors "dynatron.me/x/stillbox/pkg/nexus/errors"
	"dynatron.me/x/stillbox/pkg/pb"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
	"github.com/gohugoio/hashstructure"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
)

var (
	ErrNoWorkers = errors.New("no workers")
)

type Client interface {
	Send(broadcast.ToClientMsg) error
}

type Manager interface {
	// Broadcast broadcasts this message if it passes filtering.
	Dispatch(ctx context.Context, msg broadcast.Message) error

	// Broadcast broadcasts this message if it passes filtering.
	DispatchUnfiltered(ctx context.Context, msg broadcast.Message) error

	// Register registers a client to the pool.
	Register(ctx context.Context, c Client, cmd *pb.Register) error

	// Unregister unregisters a client from the worker pool.
	Unregister(Client)

	// SetTranscript sets a transcript.
	SetTranscript(ctx context.Context, stx *pb.SetTranscript) error

	// HUP implements Hupper.
	HUP(config.Workers)
}

type Broadcaster interface {
	Broadcast(broadcast.Message)
}

type workerManager struct {
	sync.Mutex
	filter  *filter.Filter
	tgst    tgstore.Store
	metrics workerMetrics
	workers robin.Round[Client]
	bcaster Broadcaster
	cfg     config.Workers
	cfgHash uint64
}

func (wm *workerManager) config(cfg config.Workers) error {
	wm.cfg = cfg

	err := wm.initFilter(cfg)
	if err != nil {
		return err
	}

	return nil
}

func (wm *workerManager) initFilter(cfg config.Workers) error {
	if cfg.Filter == nil {
		return nil
	}

	filt, err := filter.FromMap(cfg.Filter)
	if err != nil {
		return err
	}

	wm.filter = filt
	wm.tgst.RegisterFilter(wm.filter)

	return nil
}

func (wm *workerManager) HUP(cfg config.Workers) {
	wm.Lock()
	defer wm.Unlock()

	hash, err := hashstructure.Hash(cfg, nil)
	if err != nil {
		log.Error().Err(err).Msg("hup transcription config hash")
		return
	}

	if hash == wm.cfgHash {
		return
	}

	wm.cfgHash = 0 // set to invalid state in case we fail

	log.Info().Msg("reloading transcription config")

	wm.tgst.UnregisterFilter(wm.filter)

	err = wm.config(cfg)
	if err != nil {
		log.Error().Err(err).Msg("hup worker config")
		return
	}

	wm.cfgHash = hash
}

type workerMetrics struct {
	NoWorkersCount prometheus.Counter   `help:"Transcript calls with no workers count"`
	DispatchCount  prometheus.Counter   `help:"Dispatched transcriptions"`
	ElapsedSeconds prometheus.Histogram `help:"Transcription elapsed time" buckets:"0.1,0.2,0.5,1,1.5,2,5,10,20,50"`
}

func (wm *workerManager) SetTranscript(ctx context.Context, stx *pb.SetTranscript) error {
	callID, err := uuid.Parse(stx.GetId())
	if err != nil {
		return err
	}

	tsc, err := callstore.FromCtx(ctx).UpdateTranscription(ctx, callID, common.PtrTo(stx.GetTranscript()))
	if err != nil {
		return err
	}

	wm.bcaster.Broadcast(tsc)

	return nil
}

func NewWorkerManager(met metrics.Metrics, nexus Broadcaster, tgst tgstore.Store, cfg config.Workers) (*workerManager, error) {
	cfgHash, err := hashstructure.Hash(cfg, nil)
	if err != nil {
		return nil, err
	}

	wm := &workerManager{
		workers: robin.New[Client](),
		bcaster: nexus,
		cfg:     cfg,
		cfgHash: cfgHash,
		tgst:    tgst,
	}

	err = wm.config(cfg)
	if err != nil {
		return nil, err
	}

	met.Register("workers", &wm.metrics)

	return wm, nil
}

func (wm *workerManager) Unregister(client Client) {
	wm.workers.Delete(client)
}

func (wm *workerManager) Register(ctx context.Context, client Client, cmd *pb.Register) error {
	err := wm.workers.Add(client)
	if err == robin.ErrExists {
		return fmt.Errorf("already registered")
	}

	return nil
}

func (wm *workerManager) TranscribeDuration(t time.Duration) {
	wm.metrics.ElapsedSeconds.Observe(t.Seconds())
}

func (wm *workerManager) dispatch(msg broadcast.Message) error {
	message := msg.ToPBMessage()
	var lastClient Client

	for {
		cl := wm.workers.Next()
		if cl == lastClient || cl == nil {
			return ErrNoWorkers
		}

		lastClient = cl

		switch err := cl.Send(message); err {
		case nxerrors.ErrSentToClosed:
			wm.workers.Delete(cl)
		case nil:
			return nil
		default:
			log.Error().Err(err).Msg("worker send failed")
		}
	}
}

func (wm *workerManager) DispatchUnfiltered(ctx context.Context, msg broadcast.Message) error {
	return wm.dispatch(msg)
}

type Durationer interface {
	GetDuration() time.Duration
}

func (wm *workerManager) Dispatch(ctx context.Context, msg broadcast.Message) error {
	wm.Lock()
	defer wm.Unlock()

	var duration time.Duration
	if dur, isDurationer := msg.(Durationer); isDurationer {
		duration = dur.GetDuration()
	}

	if !msg.BroadcastType().Has(broadcast.BcastCall) ||
		!msg.ShouldStore() ||
		duration > time.Duration(wm.cfg.AtLeastSeconds)*time.Second ||
		!wm.filter.Test(ctx, msg) {
		return nil
	}

	err := wm.dispatch(msg)
	switch err {
	case ErrNoWorkers:
		wm.metrics.NoWorkersCount.Inc()
	case nil:
	default:
		return err
	}

	wm.metrics.DispatchCount.Add(1.0)

	return nil
}
