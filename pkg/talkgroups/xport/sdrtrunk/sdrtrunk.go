package sdrtrunk

import (
	"context"
	"encoding/xml"
	"io"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/talkgroups"
)

type Playlist struct {
	XMLName  xml.Name  `xml:"playlist"`
	Version  int       `xml:"version,attr"`
	Aliases  []Alias   `xml:"alias"`
	Channels []Channel `xml:"channel,omitempty"`
	Streams  []Stream  `xml:"stream,omitempty"`
}

type Alias struct {
	XMLName  xml.Name `xml:"alias"`
	Name     string   `xml:"name,attr,omitempty"`
	Color    int      `xml:"color,attr,omitempty"`
	Group    string   `xml:"group,attr,omitempty"`
	IconName string   `xml:"iconName,attr,omitempty"`
	List     string   `xml:"list,attr,omitempty"`
	IDs      []ID     `xml:"id"`
}

func tgToAlias(tg *talkgroups.Talkgroup) Alias {
	return Alias{
		XMLName: xml.Name{Local: "alias"},
		Name:    common.ZeroOr(tg.Name),
		Group:   common.ZeroOr(tg.TGGroup),
		List:    "Stillbox",
		IDs: []ID{
			ID{
				XMLName: xml.Name{Local: "id"},
				Type:    "talkgroup",
				Value:   common.PtrTo(int(tg.TGID)),
			},
		},
	}
}

type ID struct {
	XMLName  xml.Name `xml:"id"`
	Type     string   `xml:"type,attr"`
	Priority *int     `xml:"priority,attr,omitempty"`
	Channel  *string  `xml:"channel,attr,omitempty"`
	Protocol *string  `xml:"protocol,attr,omitempty"`
	Value    *int     `xml:"value,attr,omitempty"`
	Min      *int     `xml:"min,attr,omitempty"`
	Max      *int     `xml:"max,attr,omitempty"`
}

type Channel struct {
	XMLName xml.Name `xml:"channel"`
	Name    string   `xml:"name,attr"`
	System  string   `xml:"system,attr"`
	Enabled bool     `xml:"enabled,attr"`
	Site    string   `xml:"site,attr"`
	Order   int      `xml:"order,attr"`

	AliasListName   string          `xml:"alias_list_name"`
	EventLogConfig  EventLogConfig  `xml:"event_log_configuration"`
	SourceConfig    SourceConfig    `xml:"source_configuration"`
	AuxDecodeConfig AuxDecodeConfig `xml:"aux_decode_configuration"`
	DecodeConfig    DecodeConfig    `xml:"decode_configuration"`
	RecordConfig    RecordConfig    `xml:"record_configuration"`
}

type EventLogConfig struct {
	EventLogConfig []byte `xml:",innerxml"`
}

type SourceConfig struct {
	SourceConfig []byte `xml:",innerxml"`
}

type AuxDecodeConfig struct {
	AuxDecodeConfig []byte `xml:",innerxml"`
}

type DecodeConfig struct {
	DecodeConfig []byte `xml:",innerxml"`
}

type RecordConfig struct {
	RecordConfig []byte `xml:",innerxml"`
}

type Stream struct {
	Attributes []xml.Attr `xml:",any,attr"`
	Stream     []byte     `xml:",innerxml"`
}

func New() *Driver {
	return new(Driver)
}

type Driver struct{}

func (st *Driver) ExportTalkgroups(ctx context.Context, w io.Writer, tgs []*talkgroups.Talkgroup, tmpl []byte) error {
	var pl Playlist

	if tmpl != nil {
		err := xml.Unmarshal(tmpl, &pl)
		if err != nil {
			return err
		}

		pl.Aliases = nil
	}

	for _, tg := range tgs {
		pl.Aliases = append(pl.Aliases, tgToAlias(tg))
	}

	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	err := enc.Encode(&pl)
	if err != nil {
		return err
	}

	return enc.Close()
}
