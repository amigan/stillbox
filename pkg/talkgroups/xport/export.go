package xport

import (
	"context"
	"io"

	"dynatron.me/x/stillbox/pkg/talkgroups"
	"dynatron.me/x/stillbox/pkg/talkgroups/filter"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
	"dynatron.me/x/stillbox/pkg/talkgroups/xport/sdrtrunk"
)

type Exporter interface {
	ExportTalkgroups(ctx context.Context, w io.Writer, tgs []*talkgroups.Talkgroup, tmpl []byte) error
}

type ExportJob struct {
	Type     Format `json:"type"`
	SystemID int    `json:"systemID"`
	Template []byte `json:"template"`

	filter.TalkgroupFilter
	Exporter
}

func (ej *ExportJob) Export(ctx context.Context, w io.Writer) error {
	var tgs []*talkgroups.Talkgroup
	var err error
	tgst := tgstore.FromCtx(ctx)
	if ej.TalkgroupFilter.IsEmpty() {
		tgs, err = tgst.SystemTGs(ctx, int32(ej.SystemID))
		if err != nil {
			return err
		}
	} else {
		ids, err := ej.TalkgroupFilter.TGs(ctx)
		if err != nil {
			return err
		}
		tgs, err = tgst.TGs(ctx, ids)
		if err != nil {
			return err
		}
	}

	switch ej.Type {
	case FormatSDRTrunk:
		ej.Exporter = sdrtrunk.New()
	default:
		return ErrBadType
	}

	return ej.ExportTalkgroups(ctx, w, tgs, ej.Template)
}
