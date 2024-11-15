package talkgroups

import (
	"fmt"

	"dynatron.me/x/stillbox/pkg/database"
)

type Talkgroup struct {
	database.Talkgroup
	System  database.System `json:"system"`
	Learned bool            `json:"learned"`
}

type Metadata map[string]interface{}

type Names struct {
	System    string
	Talkgroup string
}

type ID struct {
	System    uint32 `json:"sys"`
	Talkgroup uint32 `json:"tg"`
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
