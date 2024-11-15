package nexus

import (
	"context"

	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/pb"
	"dynatron.me/x/stillbox/pkg/talkgroups"

	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/structpb"
)

type CommandIDKeyT int64

const CommandIDKey CommandIDKeyT = 0

func CtxWithCID(ctx context.Context, cmd *pb.Command) context.Context {
	return context.WithValue(ctx, CommandIDKey, cmd.CommandId)
}

func CommandID(ctx context.Context) *int64 {
	id, has := ctx.Value(CommandIDKey).(*int64)
	if !has {
		return nil
	}

	return id
}

func (c *client) HandleCommand(ctx context.Context, cmd *pb.Command) {
	ctx = CtxWithCID(ctx, cmd)
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
	_ = c.Send(e)
}

func (c *client) Talkgroup(ctx context.Context, tg *pb.Talkgroup) error {
	tgi, err := talkgroups.StoreFrom(ctx).TG(ctx, talkgroups.TG(tg.System, tg.Talkgroup))
	if err != nil {
		if err != talkgroups.ErrNotFound {
			log.Error().Err(err).Int32("sys", tg.System).Int32("tg", tg.Talkgroup).Msg("get talkgroup fail")
		}
		return err
	}

	var md *structpb.Struct
	if len(tgi.Talkgroup.Metadata) > 0 {
		md, err = structpb.NewStruct(tgi.Talkgroup.Metadata)
		if err != nil {
			log.Error().Err(err).Int32("sys", tg.System).Int32("tg", tg.Talkgroup).Msg("new pb struct for tg metadata")
		}
	}

	resp := &pb.TalkgroupInfo{
		Tg:         tg,
		Name:       tgi.Talkgroup.Name,
		Group:      tgi.Talkgroup.TgGroup,
		Frequency:  tgi.Talkgroup.Frequency,
		Metadata:   md,
		Tags:       tgi.Talkgroup.Tags,
		Learned:    tgi.Learned,
		AlphaTag:   tgi.Talkgroup.AlphaTag,
		SystemName: tgi.System.Name,
	}

	_ = c.Send(&pb.Message{
		ToClientMessage: &pb.Message_Response{
			Response: &pb.CommandResponse{
				CommandId: CommandID(ctx),
				CommandResponse: &pb.CommandResponse_TgInfo{
					TgInfo: resp,
				},
			},
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
