package filter

import (
	"context"
	"fmt"

	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/pb"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"

	tgsp "dynatron.me/x/stillbox/pkg/talkgroups"

	"github.com/go-viper/mapstructure/v2"
)

type TalkgroupFilter struct {
	Talkgroups       []tgsp.ID `json:"talkgroups,omitempty" form:"talkgroups"`
	TalkgroupsNot    []tgsp.ID `json:"talkgroupsNot,omitempty" form:"talkgroupsNot"`
	TalkgroupTagsAll []string  `json:"talkgroupTagsAll,omitempty" form:"talkgroupTagsAll"`
	TalkgroupTagsAny []string  `json:"talkgroupTagsAny,omitempty" form:"talkgroupTagsAny"`
	TalkgroupTagsNot []string  `json:"talkgroupTagsNot,omitempty" form:"talkgroupTagsNot"`

	talkgroups map[tgsp.ID]bool `json:"-"`
}

func (f *TalkgroupFilter) TGs(ctx context.Context) (tgsp.IDs, error) {
	err := f.ensureCompiled(ctx)
	if err != nil {
		return nil, err
	}

	r := make(tgsp.IDs, 0, len(f.talkgroups))
	for tg := range f.talkgroups {
		r = append(r, tg)
	}

	return r, nil
}

func (f *TalkgroupFilter) Tuples(ctx context.Context) (database.TGTuples, error) {
	err := f.ensureCompiled(ctx)
	if err != nil {
		return database.TGTuples{}, err
	}

	sys := make([]uint32, len(f.talkgroups))
	tgs := make([]uint32, len(f.talkgroups))

	i := 0
	for tg := range f.talkgroups {
		sys[i] = tg.System
		tgs[i] = tg.Talkgroup
	}

	return database.TGTuples{sys, tgs}, nil
}

func (f *TalkgroupFilter) ensureCompiled(ctx context.Context) error {
	if f.talkgroups == nil {
		return f.compile(ctx)
	}

	return nil
}

func (tgf *TalkgroupFilter) IsEmpty() bool {
	if tgf == nil {
		return true
	}

	if len(tgf.Talkgroups) > 0 ||
		len(tgf.TalkgroupsNot) > 0 ||
		len(tgf.TalkgroupTagsAll) > 0 ||
		len(tgf.TalkgroupTagsAny) > 0 ||
		len(tgf.TalkgroupsNot) > 0 {
		return false
	}

	return true
}

func FromMap(m map[string]any) (*TalkgroupFilter, error) {
	filter := new(TalkgroupFilter)
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Metadata:         nil,
		Result:           &filter,
		TagName:          "yaml",
		WeaklyTypedInput: true,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.TextUnmarshallerHookFunc(),
		),
	})
	if err != nil {
		return nil, err
	}
	err = dec.Decode(m)
	if err != nil {
		return nil, err
	}

	return filter, nil
}

func FromProtobuf(ctx context.Context, p *pb.Filter) (*TalkgroupFilter, error) {
	tgf := &TalkgroupFilter{
		TalkgroupTagsAll: p.TalkgroupTagsAll,
		TalkgroupTagsAny: p.TalkgroupTagsAny,
		TalkgroupTagsNot: p.TalkgroupTagsNot,
	}

	if l := len(p.Talkgroups); l > 0 {
		tgf.Talkgroups = make([]tgsp.ID, l)
		for i, t := range p.Talkgroups {
			tgf.Talkgroups[i] = tgsp.ID{
				System:    uint32(t.System),
				Talkgroup: uint32(t.Talkgroup),
			}
		}
	}

	if l := len(p.TalkgroupsNot); l > 0 {
		tgf.TalkgroupsNot = make([]tgsp.ID, l)
		for i, t := range p.TalkgroupsNot {
			tgf.TalkgroupsNot[i] = tgsp.ID{
				System:    uint32(t.System),
				Talkgroup: uint32(t.Talkgroup),
			}
		}
	}

	return tgf, tgf.compile(ctx)
}

func (f *TalkgroupFilter) hasTags() bool {
	return len(f.TalkgroupTagsAny) > 0 || len(f.TalkgroupTagsAll) > 0 || len(f.TalkgroupTagsNot) > 0
}

func (f *TalkgroupFilter) GetFinalTalkgroups() map[tgsp.ID]bool {
	return f.talkgroups
}

func (f *TalkgroupFilter) compile(ctx context.Context) error {
	f.talkgroups = make(map[tgsp.ID]bool)
	for _, tg := range f.Talkgroups {
		f.talkgroups[tg] = true
	}

	tgst := tgstore.FromCtx(ctx)

	if f.hasTags() { // don't bother with DB if no tags
		tagTGs, err := tgst.TGsByTags(ctx, f.TalkgroupTagsAll, f.TalkgroupTagsAny, f.TalkgroupTagsNot)
		if err != nil {
			return fmt.Errorf("tgsbytags: %w", err)
		}

		for _, tg := range tagTGs {
			f.talkgroups[tg] = true
		}
	}

	for _, tg := range f.TalkgroupsNot {
		f.talkgroups[tg] = false
	}

	return nil
}

func (f *TalkgroupFilter) Test(ctx context.Context, call *calls.Call) bool {
	if f == nil { // no filter means all calls
		return true
	}

	err := f.ensureCompiled(ctx)
	if err != nil {
		panic(err)
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
