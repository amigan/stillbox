package xport

import (
	"bytes"
	"context"
	"io"

	"dynatron.me/x/stillbox/pkg/talkgroups"
	"dynatron.me/x/stillbox/pkg/talkgroups/xport/radioref"
)

type Importer interface {
	ImportTalkgroups(ctx context.Context, sys int, r io.Reader) ([]talkgroups.Talkgroup, error)
}

type ImportJob struct {
	Type     Format `json:"type"`
	SystemID int    `json:"systemID"`
	Body     string `json:"body"`

	Importer `json:"-"`
}

func (ij *ImportJob) Import(ctx context.Context) ([]talkgroups.Talkgroup, error) {
	r := bytes.NewReader([]byte(ij.Body))

	switch ij.Type {
	case FormatRadioReference:
		ij.Importer = radioref.New()
	default:
		return nil, ErrBadType
	}

	return ij.ImportTalkgroups(ctx, ij.SystemID, r)
}
