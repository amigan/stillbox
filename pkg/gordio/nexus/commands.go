package nexus

import (
	"context"
	"encoding/json"

	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/gordio/database"
	"dynatron.me/x/stillbox/pkg/pb"

	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/structpb"
)

func (c *client) HandleCommand(ctx context.Context, cmd *pb.Command) {
	var err error
	switch cc := cmd.Command.(type) {
	case *pb.Command_LiveCommand:
		err = c.Live(ctx, cc.LiveCommand)
	case *pb.Command_SearchCommand:
	case *pb.Command_TgCommand:
		err = c.Talkgroup(ctx, cc.TgCommand)
	default:
		log.Error().Msgf("unknown command %T", cmd)
	}

	if err != nil {
		c.SendError(cmd, err)
	}
}

func (c *client) SendError(cmd *pb.Command, err error) {
	e := &pb.Message{
		ToClientMessage: &pb.Message_Error{
			Error: &pb.Error{
				Error:   err.Error(),
				Command: cmd,
			},
		},
	}
	c.Send(e)
}

func (c *client) Talkgroup(ctx context.Context, tg *pb.Talkgroup) error {
	db := database.FromCtx(ctx)
	tgi, err := db.GetTalkgroup(ctx, int(tg.System), int(tg.Talkgroup))
	if err != nil {
		return err
	}

	var md *structpb.Struct
	if len(tgi.Metadata) > 0 {
		m := make(map[string]interface{})
		err := json.Unmarshal(tgi.Metadata, m)
		if err != nil {
			log.Error().Err(err).Int32("sys", tg.System).Int32("tg", tg.Talkgroup).Msg("unmarshal tg metadata")
		}
		md, err = structpb.NewStruct(m)
		if err != nil {
			log.Error().Err(err).Int32("sys", tg.System).Int32("tg", tg.Talkgroup).Msg("new pb struct for tg metadata")
		}
	}

	resp := &pb.TalkgroupInfo{
		Tg:        tg,
		Name:      tgi.Name,
		Group:     tgi.TgGroup,
		Frequency: tgi.Frequency,
		Tags:      tgi.Tags,
		Metadata:  md,
	}

	c.Send(&pb.Message{
		ToClientMessage: &pb.Message_TgInfo{
			TgInfo: resp,
		},
	})

	return nil
}

func (c *client) Live(ctx context.Context, cmd *pb.Live) error {
	c.Lock()
	defer c.Unlock()

	if cmd.State != nil {
		c.liveState = *cmd.State
	}

	if cmd.Filter != nil {
		filter, err := calls.TalkgroupFilterFromPB(ctx, cmd.Filter)
		if err != nil {
			log.Error().Err(err).Msg("filter create failed")
			return err
		}

		c.filter = filter
	}

	return nil
}
