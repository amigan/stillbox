package nexus

import (
	"sync"
)

type Nexus struct {
	sync.RWMutex

	clients map[*client]struct{}

	*wsManager
}

type Registry interface {
	NewClient(Connection) Client
	Register(Client)
	Unregister(Client)
}

func New() *Nexus {
	n := &Nexus{
		clients: make(map[*client]struct{}),
	}

	n.wsManager = newWsManager(n)

	return n
}

func (n *Nexus) Register(c Client) {
	n.Lock()
	defer n.Unlock()

	n.clients[c.(*client)] = struct{}{}
}

func (n *Nexus) Unregister(c Client) {
	n.Lock()
	defer n.Unlock()

	delete(n.clients, c.(*client))
}
