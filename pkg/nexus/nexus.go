package nexus

import (
	"context"
	"sync"

	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/metrics"
	"dynatron.me/x/stillbox/pkg/nexus/broadcast"
	"dynatron.me/x/stillbox/pkg/pb"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
)

type nexus struct {
	sync.RWMutex

	tgst tgstore.Store

	clients map[*client]struct{}

	*wsManager

	bcastChan chan Message

	metrics nexMetrics
}

type nexMetrics struct {
	ActiveWSConns prometheus.Gauge `help:"Number of active websocket connections."`
}

type Nexus interface {
	Broadcast(Message)
	PrivateRoutes(chi.Router)
	Go(ctx context.Context)
}

var _ Nexus = (*nexus)(nil)

type Registry interface {
	NewClient(Connection, entities.Subject) Client
	Register(Client)
	Unregister(Client)
}

func New(tgst tgstore.Store, met metrics.Metrics) *nexus {
	n := &nexus{
		clients:   make(map[*client]struct{}),
		bcastChan: make(chan Message),
		tgst:      tgst,
	}

	n.wsManager = newWsManager(n)

	met.Register("nexus", &n.metrics)

	return n
}

func (n *nexus) Go(ctx context.Context) {
	ctx = entities.CtxWithServiceSubject(ctx, "nexus")
	for {
		select {
		case msg, ok := <-n.bcastChan:
			if !ok {
				return
			}

			n.broadcastToClients(ctx, msg)
		case <-ctx.Done():
			n.Shutdown()
			return
		}
	}
}

type Message interface {
	ToPBMessage() *pb.Message
	BroadcastType() broadcast.Type
	broadcast.Envelope
}

func (n *nexus) Broadcast(msg Message) {
	n.bcastChan <- msg
}

func (n *nexus) broadcastToClients(ctx context.Context, msg Message) {
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
		case ErrSentToClosed:
			// we already hold the lock, and the channel is closed anyway
			delete(n.clients, cl)
		case nil:
		default:
			log.Error().Err(err).Msg("broadcast send failed")
		}
		cl.RUnlock()
	}
}

func (n *nexus) Register(c Client) {
	n.Lock()
	defer n.Unlock()

	n.clients[c.(*client)] = struct{}{}
	n.metrics.ActiveWSConns.Inc()
}

func (n *nexus) Unregister(c Client) {
	n.Lock()
	defer n.Unlock()

	cl := c.(*client)
	if cl.filter != nil {
		n.tgst.UnregisterFilter(cl.filter)
	}

	delete(n.clients, cl)
	n.metrics.ActiveWSConns.Dec()
}

func (n *nexus) Shutdown() {
	n.Lock()
	defer n.Unlock()

	close(n.bcastChan)

	for c := range n.clients {
		c.Shutdown()
	}
}
