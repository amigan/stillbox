package nexus

import (
	"dynatron.me/x/stillbox/pkg/pb"

	"github.com/rs/zerolog/log"
)

func (c *client) HandleCommand(cmd *pb.Command) {
	switch cc := cmd.Command.(type) {
	case *pb.Command_LiveCommand:
		c.Live(cc.LiveCommand)
	case *pb.Command_SearchCommand:
	default:
		log.Error().Msgf("unknown command %T", cmd)
	}
}

func (c *client) Live(cmd *pb.Live) {
	if cmd.State != nil {
		c.liveState = *cmd.State
	}
}
