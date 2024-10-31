package calls

import (
	"context"

	"dynatron.me/x/stillbox/pkg/gordio/database"
)

type Talkgroup struct {
	System    uint32
	Talkgroup uint32
}

func (c *Call) TalkgroupTuple() Talkgroup {
	return Talkgroup{System: uint32(c.System), Talkgroup: uint32(c.Talkgroup)}
}

func TG[T int | uint | int64 | uint64 | int32 | uint32](sys, tgid T) Talkgroup {
	return Talkgroup{
		System:    uint32(sys),
		Talkgroup: uint32(tgid),
	}
}

func (t Talkgroup) Pack() int64 {
	// P25 system IDs are 12 bits, so we can fit them in a signed 8 byte int (int64, pg INT8)
	return int64((int64(t.System) << 32) | int64(t.Talkgroup))
}

func PackedTGs(tg []Talkgroup) []int64 {
	s := make([]int64, len(tg))

	for i, v := range tg {
		s[i] = v.Pack()
	}

	return s
}

type tgMap map[Talkgroup]database.GetTalkgroupWithLearnedByPackedIDsRow
type TalkgroupCache struct {
	tgs     tgMap
	systems map[int32]string
}

func NewTalkgroupCache(ctx context.Context, packedTgs []int64) (*TalkgroupCache, error) {
	tgc := &TalkgroupCache{
		tgs:     make(tgMap),
		systems: make(map[int32]string),
	}

	return tgc, tgc.LoadTGs(ctx, packedTgs)
}

func (t *TalkgroupCache) LoadTGs(ctx context.Context, packedTgs []int64) error {
	tgRecords, err := database.FromCtx(ctx).GetTalkgroupWithLearnedByPackedIDs(ctx, packedTgs)
	if err != nil {
		return err
	}

	for _, rec := range tgRecords {
		t.tgs[TG(rec.SystemID, rec.Tgid)] = rec
		t.systems[rec.SystemID] = rec.SystemName
	}

	return nil
}

func (t *TalkgroupCache) TG(tg Talkgroup) (database.GetTalkgroupWithLearnedByPackedIDsRow, bool) {
	rec, has := t.tgs[tg]

	return rec, has
}

func (t *TalkgroupCache) SystemName(id int) (string, bool) {
	n, has := t.systems[int32(id)]
	return n, has
}
