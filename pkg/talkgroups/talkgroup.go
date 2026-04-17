package talkgroups

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/database"
	"gopkg.in/yaml.v3"
)

type Talkgroup struct {
	database.Talkgroup
	System  database.System `json:"system"`
	Learned bool            `json:"learned"`
}

func (t *Talkgroup) GetResourceName() string {
	return entities.ResourceTalkgroup
}

func (t Talkgroup) String() string {
	if t.System.Name == "" {
		t.System.Name = strconv.Itoa(int(t.Talkgroup.TGID))
	}

	if t.Talkgroup.Name != nil || t.Talkgroup.TGGroup != nil || t.Talkgroup.AlphaTag != nil {
		return t.System.Name + " " + t.Talkgroup.String()
	}

	return fmt.Sprintf("%s:%d", t.System.Name, int(t.Talkgroup.TGID))
}

type Metadata map[string]any

type ID struct {
	System    uint32 `json:"sys"`
	Talkgroup uint32 `json:"tg"`
}

type PresenceMap map[ID]struct{}

func (t PresenceMap) Has(id ID) bool {
	_, has := t[id]

	return has
}

func (t PresenceMap) Put(id ID) {
	t[id] = struct{}{}
}

var _ encoding.TextUnmarshaler = (*ID)(nil)

var ErrBadTG = errors.New("bad talkgroup format")

func (tid *ID) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%d:%d"`, tid.System, tid.Talkgroup)), nil
}

func (tid *ID) UnmarshalJSON(j []byte) error {
	// this is all a dirty hack since we cannot implement both TextUnmarshaler
	// and json.Unmarshaler at the same time. sigh.

	// at least quote, numeral, quote at the bare minimum
	if len(j) < 3 {
		return ErrBadTG
	}

	// check if it's a string
	if j[0] == '"' && j[len(j)-1] == '"' {
		var str string
		err := json.Unmarshal(j, &str)
		if err != nil {
			return err
		}

		return tid.UnmarshalText([]byte(str))
	}

	v := &struct {
		System    uint32 `json:"sys"`
		Talkgroup uint32 `json:"tg"`
	}{}

	if tid != nil {
		v.System, v.Talkgroup = tid.System, tid.Talkgroup
	}

	err := json.Unmarshal(j, v)
	if err != nil {
		return err
	}

	tid.System, tid.Talkgroup = v.System, v.Talkgroup

	return nil
}

func (id *ID) UnmarshalText(txt []byte) error {
	ar := strings.Split(string(txt), ":")

	var err error
	switch len(ar) {
	case 2:
		id.System, err = common.AtoiU32(ar[0])
		if err != nil {
			return err
		}
		fallthrough
	case 1:
		id.Talkgroup, err = common.AtoiU32(ar[len(ar)-1])
		if err != nil {
			return err
		}
	default:
		return ErrBadTG
	}

	return nil
}

func (id *ID) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		return id.UnmarshalText([]byte(node.Value))
	case yaml.MappingNode:
		type alias ID
		noMethods := (*alias)(id)
		return node.Decode(&noMethods)
	default:
		return ErrBadTG
	}
}

type IDs []ID

func (t IDs) Tuples() database.TGTuples {
	sys := make([]uint32, len(t))
	tg := make([]uint32, len(t))

	for i := range t {
		sys[i] = t[i].System
		tg[i] = t[i].Talkgroup
	}

	return database.TGTuples{sys, tg}
}

type intId interface {
	int | uint | int64 | uint64 | int32 | uint32
}

func TG[T intId, U intId](sys T, tgid U) ID {
	return ID{
		System:    uint32(sys),
		Talkgroup: uint32(tgid),
	}
}

func (t ID) String() string {
	return fmt.Sprintf("%d:%d", t.System, t.Talkgroup)

}
