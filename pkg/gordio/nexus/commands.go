package nexus

import (
	"context"

	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/pb"

	"github.com/rs/zerolog/log"
)

func (c *client) HandleCommand(ctx context.Context, cmd *pb.Command) {
	switch cc := cmd.Command.(type) {
	case *pb.Command_LiveCommand:
		c.Live(ctx, cc.LiveCommand)
	case *pb.Command_SearchCommand:
	default:
		log.Error().Msgf("unknown command %T", cmd)
	}
}

func (c *client) SendError(err error) {
	e := &pb.Message{
		ToClientMessage: &pb.Message_Error{
			Error: &pb.Error{
				Error: err.Error(),
			},
		},
	}
	c.Send(e)
}

func (c *client) Live(ctx context.Context, cmd *pb.Live) {
	c.Lock()
	defer c.Unlock()

	if cmd.State != nil {
		c.liveState = *cmd.State
	}

	if cmd.Filter != nil {
		filter, err := calls.TalkgroupFilterFromPB(ctx, cmd.Filter)
		if err != nil {
			log.Error().Err(err).Msg("filter create failed")
			c.SendError(err)
			return
		}

		c.filter = filter
	}
}
