package nexus

import (
	"context"
	"io"
	"runtime"
	"sync"

	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/gordio/version"
	"dynatron.me/x/stillbox/pkg/pb"

	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"
)

type Client interface {
	sync.Locker

	Connection

	Hello()
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

func pbVersion() *pb.Version {
	return &pb.Version{
		ServerName: version.Name,
		Version:    version.Version,
		Built:      version.Built,
		Platform:   runtime.GOOS + "-" + runtime.GOARCH,
	}
}

func (c *client) Hello() {
	c.Send(&pb.Message{
		ToClientMessage: &pb.Message_Hello{
			Hello: &pb.Hello{
				Version: pbVersion(),
			},
		},
	})
}
