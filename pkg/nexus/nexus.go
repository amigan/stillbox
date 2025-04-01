package nexus

import (
	"context"
	"sync"

	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/pb"
	"dynatron.me/x/stillbox/pkg/rbac/entities"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"

	"github.com/rs/zerolog/log"
)

type Nexus struct {
	sync.RWMutex

	tgst tgstore.Store

	clients map[*client]struct{}

	*wsManager

	callCh chan *calls.Call
}

type Registry interface {
	NewClient(Connection, entities.Subject) Client
	Register(Client)
	Unregister(Client)
}

func New(tgst tgstore.Store) *Nexus {
	n := &Nexus{
		clients: make(map[*client]struct{}),
		callCh:  make(chan *calls.Call),
		tgst:    tgst,
	}

	n.wsManager = newWsManager(n)

	return n
}

func (n *Nexus) Go(ctx context.Context) {
	ctx = entities.CtxWithServiceSubject(ctx, "nexus")
	for {
		select {
		case call, ok := <-n.callCh:
			if !ok {
				return
			}

			n.broadcastCallToClients(ctx, call)
		case <-ctx.Done():
			n.Shutdown()
			return
		}
	}
}

func (n *Nexus) BroadcastCall(call *calls.Call) {
	n.callCh <- call
}

func (n *Nexus) broadcastCallToClients(ctx context.Context, call *calls.Call) {
	message := &pb.Message{
		ToClientMessage: &pb.Message_Call{Call: call.ToPB()},
	}
	n.Lock()
	defer n.Unlock()

	for cl := range n.clients {
		clientCtx := entities.CtxWithSubject(ctx, cl.subject)
		if !cl.filter.Test(clientCtx, call) {
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
	}
}

func (n *Nexus) Register(c Client) {
	n.Lock()
	defer n.Unlock()

	n.clients[c.(*client)] = struct{}{}
}

func (n *Nexus) Unregister(c Client) {
	n.Lock()
	defer n.Unlock()

	cl := c.(*client)
	if cl.filter != nil {
		n.tgst.UnregisterFilter(cl.filter)
	}

	delete(n.clients, cl)
}

func (n *Nexus) Shutdown() {
	n.Lock()
	defer n.Unlock()

	close(n.callCh)

	for c := range n.clients {
		c.Shutdown()
	}
}
