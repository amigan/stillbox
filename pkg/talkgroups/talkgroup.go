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

type ID struct {
	System    uint32
	Talkgroup uint32
}

func TG[T int | uint | int64 | uint64 | int32 | uint32](sys, tgid T) ID {
	return ID{
		System:    uint32(sys),
		Talkgroup: uint32(tgid),
	}
}

func (t ID) Pack() int64 {
	// P25 system IDs are 12 bits, so we can fit them in a signed 8 byte int (int64, pg INT8)
	return int64((int64(t.System) << 32) | int64(t.Talkgroup))
}

func (t ID) String() string {
	return fmt.Sprintf("%d:%d", t.System, t.Talkgroup)
}

func PackedTGs(tg []ID) []int64 {
	s := make([]int64, len(tg))

	for i, v := range tg {
		s[i] = v.Pack()
	}

	return s
}
