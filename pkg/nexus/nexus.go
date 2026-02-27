package nexus

import (
	"context"
	"sync"

	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/metrics"
	"dynatron.me/x/stillbox/pkg/nexus/broadcast"
	nxerrors "dynatron.me/x/stillbox/pkg/nexus/errors"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
	"dynatron.me/x/stillbox/pkg/workers"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
)

type nexus struct {
	sync.RWMutex

	tgst tgstore.Store

	clients map[*client]struct{}

	*wsManager

	bcastChan chan broadcast.Message

	metrics nexMetrics

	transcriptWorkers workers.Manager
}

type nexMetrics struct {
	ActiveWSConnectionCount prometheus.Gauge `help:"Number of active websocket connections"`
}

type Nexus interface {
	Broadcast(broadcast.Message)
	PrivateRoutes(chi.Router)
	Transcriber() workers.Manager
	Go(ctx context.Context)
	HUP(*config.Config)
}

var _ Nexus = (*nexus)(nil)

type Registry interface {
	NewClient(Connection, entities.Subject) *client
	Register(*client)
	Unregister(*client)
}

func (n *nexus) HUP(cfg *config.Config) {
	if n.transcriptWorkers != nil {
		n.transcriptWorkers.HUP(cfg.Transcription)
	}
}

func New(transcriptCfg config.Workers, tgst tgstore.Store, met metrics.Metrics) (*nexus, error) {
	n := &nexus{
		clients:   make(map[*client]struct{}),
		bcastChan: make(chan broadcast.Message),
		tgst:      tgst,
	}

	var err error
	n.transcriptWorkers, err = workers.NewWorkerManager(met, n, tgst, transcriptCfg)
	if err != nil {
		return nil, err
	}

	n.wsManager = newWsManager(n)

	met.Register("nexus", &n.metrics)

	return n, nil
}

func (n *nexus) Transcriber() workers.Manager {
	return n.transcriptWorkers
}

func (n *nexus) Go(ctx context.Context) {
	ctx = entities.CtxWithServiceSubject(ctx, "nexus")
	for {
		select {
		case msg := <-n.bcastChan:
			n.broadcastToClients(ctx, msg)
		case <-ctx.Done():
			n.Shutdown()
			return
		}
	}
}

func (n *nexus) Broadcast(msg broadcast.Message) {
	n.bcastChan <- msg
}

func (n *nexus) broadcastToClients(ctx context.Context, msg broadcast.Message) {
	n.Lock()
	defer n.Unlock()

	message := msg.ToPBMessage()
	bcType := msg.BroadcastType()

	for cl := range n.clients {
		cl.RLock()

		if !cl.subscriptions.Has(bcType) ||
			!cl.filter.Test(entities.CtxWithSubject(ctx, cl.subject), msg) {
			cl.RUnlock()
			continue
		}

		switch err := cl.Send(message); err {
		case nxerrors.ErrSentToClosed:
			// we already hold the lock, and the channel is closed anyway
			n.unregister(cl)
		case nil:
		default:
			log.Error().Err(err).Msg("broadcast send failed")
		}
		cl.RUnlock()
	}

	if n.transcriptWorkers != nil {
		err := n.transcriptWorkers.Dispatch(ctx, msg)
		if err != nil {
			log.Error().Err(err).Msg("could not broadcast")
		}
	}
}

func (n *nexus) Register(c *client) {
	n.Lock()
	defer n.Unlock()

	n.clients[c] = struct{}{}
	n.metrics.ActiveWSConnectionCount.Inc()
}

func (n *nexus) Unregister(c *client) {
	n.Lock()
	defer n.Unlock()

	n.unregister(c)
}

func (n *nexus) unregister(cl *client) {
	if cl.filter != nil {
		n.tgst.UnregisterFilter(cl.filter)
	}

	if cl.isTranscriptWorker && n.transcriptWorkers != nil {
		n.transcriptWorkers.Unregister(cl)
	}

	delete(n.clients, cl)
	n.metrics.ActiveWSConnectionCount.Dec()
}

func (n *nexus) Shutdown() {
	n.Lock()
	defer n.Unlock()

	for c := range n.clients {
		c.Shutdown()
	}
}
