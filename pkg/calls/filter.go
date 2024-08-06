package calls

import (
	"context"

	"dynatron.me/x/stillbox/pkg/gordio/database"
)

type FilterQuery struct {
	Query  string
	Params []interface{}
}

type Talkgroup struct {
	System    uint32
	Talkgroup uint32
}

func (t Talkgroup) Pack() int64 {
	// P25 system IDs are 12 bits, so we can fit them in a signed 8 byte int (int64, pg INT8)
	return int64((int64(t.System) << 32) | int64(t.Talkgroup))
}

type Filter struct {
	Talkgroups       []Talkgroup `json:"talkgroups,omitempty"`
	TalkgroupsNot    []Talkgroup `json:"talkgroupsNot,omitempty"`
	TalkgroupTagsAll []string    `json:"talkgroupTagsAll,omitempty"`
	TalkgroupTagsAny []string    `json:"talkgroupTagsAny,omitempty"`
	TalkgroupTagsNot []string    `json:"talkgroupTagsNot,omitempty"`

	talkgroups map[Talkgroup]bool
}

func PackedTGs(tg []Talkgroup) []int64 {
	s := make([]int64, len(tg))

	for i, v := range tg {
		s[i] = v.Pack()
	}

	return s
}

func (f *Filter) hasTags() bool {
	return len(f.TalkgroupTagsAny) > 0 || len(f.TalkgroupTagsAll) > 0 || len(f.TalkgroupTagsNot) > 0
}

func (f *Filter) GetFinalTalkgroups() map[Talkgroup]bool {
	return f.talkgroups
}

func (f *Filter) compile(ctx context.Context) error {
	f.talkgroups = make(map[Talkgroup]bool)
	for _, tg := range f.Talkgroups {
		f.talkgroups[tg] = true
	}

	if f.hasTags() { // don't bother with DB if no tags
		db := database.FromCtx(ctx)
		tagTGs, err := db.GetTalkgroupIDsByTags(ctx, f.TalkgroupTagsAny, f.TalkgroupTagsAll, f.TalkgroupTagsNot)
		if err != nil {
			return nil, err
		}

		for _, tg := range tagTGs {
			f.talkgroups[Talkgroup{System: uint32(tg.SystemID), Talkgroup: uint32(tg.Tgid)}] = true
		}
	}

	for _, tg := range f.TalkgroupsNot {
		f.talkgroups[tg] = false
	}

	return nil
}

func (f *Filter) Test(ctx context.Context, call *Call) bool {
	if f == nil { // no filter means all calls
		return true
	}

	if f.talkgroups == nil {
		err := f.compile(ctx)
		if err != nil {
			panic(err)
		}
	}

	return false
}
