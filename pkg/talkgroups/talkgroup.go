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

type Names struct {
	System    string
	Talkgroup string
}

type ID struct {
	System    uint32 `json:"sys"`
	Talkgroup uint32 `json:"tg"`
}

type IDs []ID

func (ids *IDs) Packed() []int64 {
	r := make([]int64, len(*ids))
	for i := range *ids {
		r[i] = (*ids)[i].Pack()
	}

	return r
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

func (t ID) Pack() int64 {
	// P25 system IDs are 12 bits, so we can fit them in a signed 8 byte int (int64, pg INT8)
	return int64((int64(t.System) << 32) | int64(t.Talkgroup))
}

func Unpack(id int64) ID {
	return ID{
		System:    uint32(id >> 32),
		Talkgroup: uint32(id & 0xffffffff),
	}
}

func (t ID) String() string {
	return fmt.Sprintf("%d:%d", t.System, t.Talkgroup)

}
