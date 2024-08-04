package nexus

import (
	"sync"

	"dynatron.me/x/stillbox/pkg/gordio/calls"
	"dynatron.me/x/stillbox/pkg/pb"

	"github.com/rs/zerolog/log"
)

type Nexus struct {
	sync.RWMutex

	clients map[*client]struct{}

	*wsManager

	callCh chan *calls.Call
}

type Registry interface {
	NewClient(Connection) Client
	Register(Client)
	Unregister(Client)
}

func New() *Nexus {
	n := &Nexus{
		clients: make(map[*client]struct{}),
		callCh:  make(chan *calls.Call),
	}

	n.wsManager = newWsManager(n)

	return n
}

func (n *Nexus) Go(done <-chan struct{}) {
	for {
		select {
		case call, ok := <-n.callCh:
			if !ok {
				return
			}

			go n.broadcastCallToClients(call)
		case <-done:
			return
		}
	}
}

func (n *Nexus) BroadcastCall(call *calls.Call) {
	n.callCh <- call
}

func (n *Nexus) broadcastCallToClients(call *calls.Call) {
	log.Info().Msg("broadcast")
	message := &pb.Message{
		ToClientMessage: &pb.Message_Call{Call: call.ToPB()},
	}
	n.RLock()
	defer n.RUnlock()

	for cl, _ := range n.clients {
		log.Info().Msg("client")
		cl.Send(message)
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
	cl.Connection.CloseCh()
	delete(n.clients, cl)
}
