package nexus

import (
	"context"
	"errors"
	"io"
	"runtime"
	"sync"

	"dynatron.me/x/stillbox/internal/version"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	tgfilter "dynatron.me/x/stillbox/pkg/calls/filter"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/nexus/broadcast"
	"dynatron.me/x/stillbox/pkg/pb"

	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type Client interface {
	sync.Locker

	Connection

	Hello(context.Context)
	HandleCommand(context.Context, *pb.Command)
	HandleMessage(context.Context, []byte)
}

type client struct {
	sync.RWMutex

	Connection

	liveState pb.LiveState
	filter    *tgfilter.Filter

	subject entities.Subject

	subscriptions broadcast.Type

	nexus *nexus
}

type ToClient interface {
	protoreflect.ProtoMessage
}

var (
	ErrSentToClosed = errors.New("sent to closed connection")
)

type Connection interface {
	io.Closer
	CloseCh()

	Shutdown()

	Send(ToClient) error
}

func (n *nexus) NewClient(conn Connection, subject entities.Subject) Client {
	sess := &client{
		Connection: conn,
		nexus:      n,
		subject:    subject,
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

func pbServerInfo(ctx context.Context) *pb.ServerInfo {
	cts, err := database.FromCtx(ctx).GetDatabaseSize(ctx)
	if err != nil {
		log.Error().Err(err).Msg("get calls table size")
		cts = "unknown"
	}

	return &pb.ServerInfo{
		ServerName: version.Name,
		Version:    version.Version,
		Built:      version.Built,
		Platform:   runtime.GOOS + "-" + runtime.GOARCH,
		DbSize:     cts,
	}
}

func (c *client) Hello(ctx context.Context) {
	_ = c.Send(&pb.Message{
		ToClientMessage: &pb.Message_Hello{
			Hello: &pb.Hello{
				ServerInfo: pbServerInfo(ctx),
			},
		},
	})
}
