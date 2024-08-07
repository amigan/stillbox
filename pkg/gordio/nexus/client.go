package nexus

import (
	"context"
	"io"
	"sync"

	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/pb"

	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"
)

type Client interface {
	sync.Locker

	Connection

	HandleCommand(context.Context, *pb.Command)
	HandleMessage(context.Context, []byte)
}

type client struct {
	sync.RWMutex

	Connection

	liveState pb.LiveState
	filter    *calls.TalkgroupFilter

	nexus *Nexus
}

type Connection interface {
	io.Closer
	CloseCh()

	Send(*pb.Message) (closed bool)
}

func (n *Nexus) NewClient(conn Connection) Client {
	sess := &client{
		Connection: conn,
		nexus:      n,
	}

	return sess
}

func (c *client) HandleMessage(ctx context.Context, mesgBytes []byte) {
	var msg pb.Command
	err := proto.Unmarshal(mesgBytes, &msg)
	if err != nil {
		log.Error().Err(err).Msg("command unmarshal")
		return
	}

	c.HandleCommand(ctx, &msg)
}
