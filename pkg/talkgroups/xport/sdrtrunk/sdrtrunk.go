package sdrtrunk

import (
	"context"
	"encoding/xml"
	"io"

	"dynatron.me/x/stillbox/pkg/talkgroups"
)

type Playlist struct {
	XMLName xml.Name `xml:"playlist"`
	Aliases []Alias  `xml:"alias"`
}

type Alias struct {
	XMLName  xml.Name `xml:"alias"`
	Group    string   `xml:"group,attr"`
	Name     string   `xml:"name,attr"`
	List     string   `xml:"list,attr"`
	Priority int      // set by us
	TGIDs    []TGID   `xml:"id"`
}

type TGID struct {
	XMLName  xml.Name `xml:"id"`
	Type     string   `xml:"type,attr"`
	Priority int      `xml:"priority,attr"`
	Channel  string   `xml:"channel,attr"`
	Value    int      `xml:"value,attr"`
	Min      int      `xml:"min,attr"`
	Max      int      `xml:"max,attr"`
}

func New() *Driver {
	return new(Driver)
}

type Driver struct{}

func (st *Driver) ImportTalkgroups(ctx context.Context, sys int, r io.Reader) ([]talkgroups.Talkgroup, error) {
	return nil, nil
}

func (st *Driver) ExportTalkgroups(ctx context.Context, w io.Writer, tgs []*talkgroups.Talkgroup) error {
	return nil
}
