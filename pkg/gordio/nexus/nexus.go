package nexus

import (
	"sync"

	"dynatron.me/x/stillbox/pkg/gordio/calls"
	"dynatron.me/x/stillbox/pkg/pb"
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
		callCh:  make(chan *calls.Call, 256),
	}

	n.wsManager = newWsManager(n)

	return n
}

func (n *Nexus) Go(wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case call, ok := <-n.callCh:
			if !ok {
				return
			}

			go n.emitCall(call)
		}
	}
}

func (n *Nexus) emitCall(call *calls.Call) {
	message := &pb.Message{
		ToClientMessage: &pb.Message_Call{Call: call.ToPB()},
	}
	n.RLock()
	defer n.RUnlock()

	for cl, _ := range n.clients {
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
	delete(n.clients, cl)
}
