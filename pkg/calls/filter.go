package calls

import (
	"context"

	"dynatron.me/x/stillbox/pkg/gordio/database"
	"dynatron.me/x/stillbox/pkg/pb"
)

type Talkgroup struct {
	System    uint32
	Talkgroup uint32
}

func (c *Call) TalkgroupTuple() Talkgroup {
	return Talkgroup{System: uint32(c.System), Talkgroup: uint32(c.Talkgroup)}
}

func (t Talkgroup) Pack() int64 {
	// P25 system IDs are 12 bits, so we can fit them in a signed 8 byte int (int64, pg INT8)
	return int64((int64(t.System) << 32) | int64(t.Talkgroup))
}

type TalkgroupFilter struct {
	Talkgroups       []Talkgroup `json:"talkgroups,omitempty"`
	TalkgroupsNot    []Talkgroup `json:"talkgroupsNot,omitempty"`
	TalkgroupTagsAll []string    `json:"talkgroupTagsAll,omitempty"`
	TalkgroupTagsAny []string    `json:"talkgroupTagsAny,omitempty"`
	TalkgroupTagsNot []string    `json:"talkgroupTagsNot,omitempty"`

	talkgroups map[Talkgroup]bool
}

func TalkgroupFilterFromPB(ctx context.Context, p *pb.Filter) (*TalkgroupFilter, error) {
	tgf := &TalkgroupFilter{
		TalkgroupTagsAll: p.TalkgroupTagsAll,
		TalkgroupTagsAny: p.TalkgroupTagsAny,
		TalkgroupTagsNot: p.TalkgroupTagsNot,
	}

	if l := len(p.Talkgroups); l > 0 {
		tgf.Talkgroups = make([]Talkgroup, l)
		for i, t := range p.Talkgroups {
			tgf.Talkgroups[i] = Talkgroup{
				System:    uint32(t.System),
				Talkgroup: uint32(t.Talkgroup),
			}
		}
	}

	if l := len(p.TalkgroupsNot); l > 0 {
		tgf.TalkgroupsNot = make([]Talkgroup, l)
		for i, t := range p.TalkgroupsNot {
			tgf.TalkgroupsNot[i] = Talkgroup{
				System:    uint32(t.System),
				Talkgroup: uint32(t.Talkgroup),
			}
		}
	}

	return tgf, tgf.compile(ctx)
}

func PackedTGs(tg []Talkgroup) []int64 {
	s := make([]int64, len(tg))

	for i, v := range tg {
		s[i] = v.Pack()
	}

	return s
}

func (f *TalkgroupFilter) hasTags() bool {
	return len(f.TalkgroupTagsAny) > 0 || len(f.TalkgroupTagsAll) > 0 || len(f.TalkgroupTagsNot) > 0
}

func (f *TalkgroupFilter) GetFinalTalkgroups() map[Talkgroup]bool {
	return f.talkgroups
}

func (f *TalkgroupFilter) compile(ctx context.Context) error {
	f.talkgroups = make(map[Talkgroup]bool)
	for _, tg := range f.Talkgroups {
		f.talkgroups[tg] = true
	}

	if f.hasTags() { // don't bother with DB if no tags
		db := database.FromCtx(ctx)
		tagTGs, err := db.GetTalkgroupIDsByTags(ctx, f.TalkgroupTagsAny, f.TalkgroupTagsAll, f.TalkgroupTagsNot)
		if err != nil {
			return err
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

func (f *TalkgroupFilter) Test(ctx context.Context, call *Call) bool {
	if f == nil { // no filter means all calls
		return true
	}

	if f.talkgroups == nil {
		err := f.compile(ctx)
		if err != nil {
			panic(err)
		}
	}

	tg := call.TalkgroupTuple()

	tgRes, have := f.talkgroups[tg]
	if have {
		return tgRes
	}

	for _, patch := range call.Patches {
		tg.Talkgroup = uint32(patch)
		tgRes, have := f.talkgroups[tg]
		if have {
			return tgRes
		}
	}

	return false
}
