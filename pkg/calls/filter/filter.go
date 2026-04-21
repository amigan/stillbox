package filter

import (
	"context"
	"fmt"
	"sync"

	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/nexus/broadcast"
	"dynatron.me/x/stillbox/pkg/pb"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"

	tgsp "dynatron.me/x/stillbox/pkg/talkgroups"

	"github.com/go-viper/mapstructure/v2"
)

type Filter struct {
	Talkgroups       tgsp.IDs `json:"talkgroups,omitempty" form:"talkgroups"`
	TalkgroupsNot    tgsp.IDs `json:"talkgroupsNot,omitempty" form:"talkgroupsNot"`
	TalkgroupTagsAll []string `json:"talkgroupTagsAll,omitempty" form:"talkgroupTagsAll"`
	TalkgroupTagsAny []string `json:"talkgroupTagsAny,omitempty" form:"talkgroupTagsAny"`
	TalkgroupTagsNot []string `json:"talkgroupTagsNot,omitempty" form:"talkgroupTagsNot"`

	All bool `json:"all,omitzero" form:"all"`

	sync.RWMutex
	talkgroups map[tgsp.ID]bool `json:"-"`
}

func (f *Filter) TGs(ctx context.Context) (tgsp.IDs, error) {
	err := f.ensureCompiled(ctx)
	if err != nil {
		return nil, err
	}

	f.RLock()
	defer f.RUnlock()
	r := make(tgsp.IDs, 0, len(f.talkgroups))
	for tg := range f.talkgroups {
		r = append(r, tg)
	}

	return r, nil
}

func (f *Filter) Tuples(ctx context.Context) (database.TGTuplesU, error) {
	err := f.ensureCompiled(ctx)
	if err != nil {
		return database.TGTuplesU{}, err
	}

	f.RLock()
	defer f.RUnlock()
	sys := make([]uint32, len(f.talkgroups))
	tgs := make([]uint32, len(f.talkgroups))

	i := 0
	for tg := range f.talkgroups {
		sys[i] = tg.System
		tgs[i] = tg.Talkgroup
	}

	return database.TGTuplesU{sys, tgs}, nil
}

func (f *Filter) ensureCompiled(ctx context.Context) error {
	if !f.All && f.talkgroups == nil {
		return f.compile(ctx)
	}

	return nil
}

func (f *Filter) Recompile(ctx context.Context) error {
	return f.compile(ctx)
}

func (f *Filter) TagRefs() []string {
	return append(f.TalkgroupTagsAll, append(f.TalkgroupTagsNot, f.TalkgroupTagsAny...)...)
}

func (f *Filter) IsEmpty() bool {
	if f == nil {
		return true
	}

	if f.All {
		return false
	}

	f.RLock()
	defer f.RUnlock()

	if len(f.Talkgroups) > 0 ||
		len(f.TalkgroupTagsAll) > 0 ||
		len(f.TalkgroupTagsAny) > 0 ||
		len(f.TalkgroupsNot) > 0 {
		return false
	}

	return true
}

func FromMap(m map[string]any) (*Filter, error) {
	filter := new(Filter)
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Metadata:         nil,
		Result:           filter,
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

func FromProtobuf(ctx context.Context, p *pb.Filter) (*Filter, error) {
	tgf := &Filter{
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

func (f *Filter) hasTags() bool {
	return len(f.TalkgroupTagsAny) > 0 || len(f.TalkgroupTagsAll) > 0 || len(f.TalkgroupTagsNot) > 0
}

func (f *Filter) compile(ctx context.Context) error {
	f.Lock()
	defer f.Unlock()

	if f.All {
		return nil
	}

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

func (f *Filter) Test(ctx context.Context, msgEnvelope broadcast.Envelope) bool {
	if f == nil || f.All { // no filter means all calls
		return true
	}

	err := f.ensureCompiled(ctx)
	if err != nil {
		panic(err)
	}

	f.RLock()
	defer f.RUnlock()

	tg := msgEnvelope.TalkgroupTuple()

	tgRes, have := f.talkgroups[tg]
	if have {
		return tgRes
	}

	for _, patch := range msgEnvelope.PatchTGs() {
		tg.Talkgroup = uint32(patch)
		tgRes, have := f.talkgroups[tg]
		if have {
			return tgRes
		}
	}

	return false
}
