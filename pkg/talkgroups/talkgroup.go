package talkgroups

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/rbac/entities"
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

type Metadata map[string]interface{}

type ID struct {
	System    uint32 `json:"sys"`
	Talkgroup uint32 `json:"tg"`
}

var _ encoding.TextUnmarshaler = (*ID)(nil)

var ErrBadTG = errors.New("bad talkgroup format")

func (tid *ID) UnmarshalJSON(j []byte) error {
	// this is a dirty hack since we cannot implement both TextUnmarshaler
	// and json.Unmarshaler at the same time. sigh.
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

func (tid *ID) UnmarshalText(txt []byte) error {
	ar := strings.Split(string(txt), ":")
	switch len(ar) {
	case 2:
		sys, err := strconv.Atoi(ar[0])
		if err != nil {
			return err
		}
		tid.System = uint32(sys)
		fallthrough
	case 1:
		tg, err := strconv.Atoi(ar[len(ar)-1])
		if err != nil {
			return err
		}
		tid.Talkgroup = uint32(tg)
	default:
		return ErrBadTG
	}

	return nil
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
