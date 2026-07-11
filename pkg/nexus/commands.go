package nexus

import (
	"context"
	"time"

	"dynatron.me/x/stillbox/pkg/authz"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/calls"
	tgfilter "dynatron.me/x/stillbox/pkg/calls/filter"
	"dynatron.me/x/stillbox/pkg/nexus/broadcast"
	"dynatron.me/x/stillbox/pkg/pb"
	"dynatron.me/x/stillbox/pkg/talkgroups"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
	"dynatron.me/x/stillbox/pkg/users"

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
	case *pb.Command_RegisterCommand:
		err = c.Register(ctx, cc.RegisterCommand)
	case *pb.Command_SearchCommand:
	case *pb.Command_TgCommand:
		err = c.Talkgroup(ctx, cc.TgCommand)
	case *pb.Command_SetTranscript:
		err = c.nexus.transcriptWorkers.SetTranscript(ctx, cc.SetTranscript)
		var elapsed time.Duration
		if cc.SetTranscript.ElapsedMs != nil {
			elapsed = time.Duration(*cc.SetTranscript.ElapsedMs) * time.Millisecond
		}
		log.Debug().Err(err).Str("call", cc.SetTranscript.Id).Str("worker", c.NetConn().RemoteAddr().String()).Str("elapsed", elapsed.String()).Msg("transcript set")
	case *pb.Command_UploadCall:
		err = c.UploadCall(ctx, cc.UploadCall)
	default:
		log.Error().Msgf("unknown command %#v", cmd)
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

// SendResponse will fill in the Command ID from the context, if present.
func (c *client) SendResponse(ctx context.Context, response *pb.CommandResponse) error {
	return c.Send(&pb.Message{
		ToClientMessage: &pb.Message_Response{
			Response: &pb.CommandResponse{
				CommandId:       CommandID(ctx),
				CommandResponse: response.CommandResponse,
			},
		},
	})
}

func (c *client) Talkgroup(ctx context.Context, tg *pb.Talkgroup) error {
	tgi, err := tgstore.FromCtx(ctx).TG(ctx, talkgroups.TG(tg.System, tg.Talkgroup))
	if err != nil {
		if err != tgstore.ErrNotFound {
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
		Group:      tgi.Talkgroup.TGGroup,
		Frequency:  tgi.Talkgroup.Frequency,
		Metadata:   md,
		Tags:       tgi.Talkgroup.Tags,
		Learned:    tgi.Learned,
		AlphaTag:   tgi.Talkgroup.AlphaTag,
		SystemName: tgi.System.Name,
	}

	return c.SendResponse(ctx, &pb.CommandResponse{
		CommandResponse: &pb.CommandResponse_TgInfo{TgInfo: resp},
	})
}

func (c *client) Register(ctx context.Context, cmd *pb.Register) error {
	if !cmd.TranscriptWorker || c.nexus.transcriptWorkers == nil {
		return nil
	}

	err := c.nexus.transcriptWorkers.Register(ctx, c, cmd)
	if err != nil {
		return err
	}

	c.isTranscriptWorker = true

	return nil
}

func (c *client) Live(ctx context.Context, cmd *pb.Live) error {
	c.Lock()
	defer c.Unlock()

	if cmd.State != nil {
		c.liveState = *cmd.State
	}

	if cmd.Filter != nil {
		filter, err := tgfilter.FromProtobuf(ctx, cmd.Filter)
		if err != nil {
			log.Error().Err(err).Msg("filter create failed")
			return err
		}

		c.filter = filter
		tgstore.FromCtx(ctx).RegisterFilter(c.filter)
	} else {
		c.filter = nil
	}

	c.subscriptions.Subscribe(cmd.Calls, broadcast.BcastCall)
	c.subscriptions.Subscribe(cmd.Transcripts, broadcast.BcastTranscription)

	return nil
}

func (c *client) UploadCall(ctx context.Context, uc *pb.UploadCall) error {
	user, err := users.UserCheck(ctx, authz.UseResource(entities.ResourceCall), entities.ActionCreate)
	if err != nil {
		return err
	}

	dontStore := false
	if uc.DontStore != nil && *uc.DontStore {
		dontStore = true
	}

	call, err := calls.FromPBCall(uc.Call, user.ID, dontStore)
	if err != nil {
		return err
	}

	return c.nexus.ing.Ingest(ctx, call)
}
