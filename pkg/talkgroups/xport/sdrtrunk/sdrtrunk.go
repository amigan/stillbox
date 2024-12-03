package sdrtrunk

import (
	"context"
	"encoding/xml"
	"io"
	"strings"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/talkgroups"
)

const (
	listName = "Stillbox"
	protocol = "APCO25" // this should be configurable
)

type Playlist struct {
	XMLName  xml.Name  `xml:"playlist"`
	Version  int       `xml:"version,attr"`
	Aliases  []Alias   `xml:"alias"`
	Channels []Channel `xml:"channel,omitempty"`
	Streams  []Stream  `xml:"stream,omitempty"`

	streams map[string]struct{}
}

func (p *Playlist) buildMaps() {
	p.streams = make(map[string]struct{})

	for i, s := range p.Streams {
		var namespace string
		for j, a := range s.Attributes {
			if strings.HasPrefix(a.Name.Local, "wstxns") {
				namespace = a.Name.Local

			}
			if a.Name.Local == "type" {
				p.Streams[i].Attributes[j].Name.Space = namespace
			}

			if a.Name.Local == "name" {
				p.streams[a.Value] = struct{}{}
			}
		}
	}

	for i := range p.Channels {
		p.Channels[i].AliasList = listName
	}
}

func (p *Playlist) HasStream(name string) bool {
	_, has := p.streams[name]

	return has
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

func (p *Playlist) tgToAlias(tg *talkgroups.Talkgroup) Alias {
	nameF := func() string {
		if tg.AlphaTag != nil {
			return *tg.AlphaTag
		}

		return common.ZeroIfNil(tg.Name)
	}
	icon, _ := tg.Metadata["icon"].(string)
	a := Alias{
		XMLName:  xml.Name{Local: "alias"},
		Name:     nameF(),
		Group:    common.ZeroIfNil(tg.TGGroup),
		List:     listName,
		IconName: icon,
		IDs: []ID{
			{
				XMLName:  xml.Name{Local: "id"},
				Type:     "talkgroup",
				Value:    common.PtrTo(int(tg.TGID)),
				Protocol: common.PtrTo(protocol),
			},
		},
	}

	// be nice and assign it to stream to ourselves
	// TODO: make this more dynamic (exporter can have options, enumerate fields into a map[string]blah?)
	// with which to specify the stillbox streamer
	if p.HasStream("stillbox") {
		a.IDs = append(a.IDs, ID{
			Type:    "broadcastChannel",
			Channel: common.PtrTo("stillbox"),
		})
	}

	return a
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

	AliasList           string          `xml:"alias_list_name"`
	EventLogConfig      EventLogConfig  `xml:"event_log_configuration"`
	SourceConfiguration SourceConfig    `xml:"source_configuration"`
	AuxDecodeConfig     AuxDecodeConfig `xml:"aux_decode_configuration"`
	DecodeConfig        DecodeConfig    `xml:"decode_configuration"`
	RecordConfig        RecordConfig    `xml:"record_configuration"`
}

type EventLogConfig struct {
	Attributes []xml.Attr `xml:",any,attr"`
	Config     []byte     `xml:",innerxml"`
}

type SourceConfig struct {
	Attributes []xml.Attr `xml:",any,attr"`
	Config     []byte     `xml:",innerxml"`
}

type AuxDecodeConfig struct {
	Attributes []xml.Attr `xml:",any,attr"`
	Config     []byte     `xml:",innerxml"`
}

type DecodeConfig struct {
	Attributes []xml.Attr `xml:",any,attr"`
	Config     []byte     `xml:",innerxml"`
}

type RecordConfig struct {
	Attributes []xml.Attr `xml:",any,attr"`
	Config     []byte     `xml:",innerxml"`
}

type Stream struct {
	XMLName    xml.Name   `xml:"stream"`
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

	pl.buildMaps()

	for _, tg := range tgs {
		pl.Aliases = append(pl.Aliases, pl.tgToAlias(tg))
	}

	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	err := enc.Encode(&pl)
	if err != nil {
		return err
	}

	return enc.Close()
}
