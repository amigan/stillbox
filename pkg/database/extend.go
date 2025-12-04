package database

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"gopkg.in/yaml.v3"
)

func (d GetTalkgroupsRow) GetTalkgroup() Talkgroup  { return d.Talkgroup }
func (d GetTalkgroupsRow) GetSystem() System        { return d.System }
func (d GetTalkgroupsRow) GetLearned() bool         { return d.Talkgroup.Learned }
func (g GetTalkgroupRow) GetTalkgroup() Talkgroup   { return g.Talkgroup }
func (g GetTalkgroupRow) GetSystem() System         { return g.System }
func (g GetTalkgroupRow) GetLearned() bool          { return g.Talkgroup.Learned }
func (g GetTalkgroupsPRow) GetTalkgroup() Talkgroup { return g.Talkgroup }
func (g GetTalkgroupsPRow) GetSystem() System       { return g.System }
func (g GetTalkgroupsPRow) GetLearned() bool        { return g.Talkgroup.Learned }
func (g Talkgroup) GetTalkgroup() Talkgroup         { return g }
func (g Talkgroup) GetSystem() System               { return System{ID: int(g.SystemID)} }
func (g Talkgroup) GetLearned() bool                { return false }

func (g Talkgroup) String() string {
	return g.StringTag(true)
}

func (g Talkgroup) StringTag(withTag bool) string {
	switch {
	case withTag && g.AlphaTag != nil:
		return *g.AlphaTag
	case g.Name != nil && g.TGGroup != nil:
		return *g.TGGroup + " " + *g.Name
	case g.Name != nil:
		return *g.Name + " [" + strconv.Itoa(int(g.TGID)) + "]"
	case g.TGGroup != nil:
		return *g.TGGroup + " [" + strconv.Itoa(int(g.TGID)) + "]"
	}

	return strconv.Itoa(int(g.TGID))
}

func (ns *NullAudioMIME) UnmarshalJSON(b []byte) error {
	return ns.Scan(b)
}

func (ns *NullAudioMIME) UnmarshalText(b []byte) error {
	return ns.Scan(b)
}

func (c *Call) UnmarshalYAML(n *yaml.Node) error {
	return unmarshalYaml(c, n)
}

func (c *IncidentsCall) UnmarshalYAML(n *yaml.Node) error {
	return unmarshalYaml(c, n)
}

func unmarshalYaml(dst any, n *yaml.Node) error {
	var m map[string]yaml.Node

	err := n.Decode(&m)
	if err != nil {
		return fmt.Errorf("unmarshal into yaml node: %w", err)
	}

	outMap := make(map[string]any, len(m))

	for k, v := range m {
		switch k {
		case "id":
			u, err := uuid.Parse(v.Value)
			if err != nil {
				return err
			}

			outMap[k] = u
		case "calls_tbl_id":
			u, err := uuid.Parse(v.Value)
			if err != nil {
				return err
			}

			outMap[k] = pgtype.UUID{Bytes: u, Valid: true}
		case "call_date":
			t, err := time.Parse(time.RFC3339Nano, v.Value)
			if err != nil {
				return err
			}
			outMap[k] = t
		case "call_audio":
			b, err := base64.StdEncoding.DecodeString(v.Value)
			if err != nil {
				return err
			}

			outMap[k] = b
		default:
			var a any

			err := v.Decode(&a)
			if err != nil {
				return err
			}

			outMap[k] = a
		}
	}

	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Metadata:         nil,
		Result:           dst,
		TagName:          "yaml",
		WeaklyTypedInput: true,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.TextUnmarshallerHookFunc(),
		),
	})
	if err != nil {
		return err
	}

	err = dec.Decode(outMap)
	if err != nil {
		return fmt.Errorf("mapstructure decode: %w", err)
	}

	return nil
}
