package xport

import (
	"context"
	"io"

	"dynatron.me/x/stillbox/pkg/calls/filter"
	"dynatron.me/x/stillbox/pkg/talkgroups"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
	"dynatron.me/x/stillbox/pkg/talkgroups/xport/sdrtrunk"
)

type Exporter interface {
	ExportTalkgroups(ctx context.Context, w io.Writer, tgs []*talkgroups.Talkgroup, tmpl []byte) error
}

type ExportJob struct {
	Type             Format `json:"type" form:"type"`
	SystemID         int    `json:"systemID" form:"systemID"`
	Template         []byte `json:"template" form:"template" filenameField:"TemplateFileName"`
	TemplateFileName string

	filter.Filter
	Exporter
}

func (ej *ExportJob) Export(ctx context.Context, w io.Writer) error {
	var tgs []*talkgroups.Talkgroup
	var err error
	tgst := tgstore.FromCtx(ctx)
	if ej.Filter.IsEmpty() {
		tgs, err = tgst.SystemTGs(ctx, ej.SystemID)
		if err != nil {
			return err
		}
	} else {
		for i, v := range ej.Filter.Talkgroups {
			if v.System == 0 {
				ej.Filter.Talkgroups[i].System = uint32(ej.SystemID)
			}
		}
		ids, err := ej.Filter.TGs(ctx)
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
