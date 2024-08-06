package calls

import (
	"cmp"
	"encoding/json"
	"slices"
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

	talkgroups       map[Talkgroup]bool
	talkgroupTagsAll map[string]bool
	talkgroupTagsAny map[string]bool
	talkgroupTagsNot map[string]bool

	query *FilterQuery
}

func queryParams(s string, p ...any) (string, []any) {
	return s, p
}

func (f *Filter) filterQuery() FilterQuery {
	var q string
	var args []interface{}

	q, args = queryParams(
		`((talkgroups.id = ANY(?) OR talkgroups.tags @> ARRAY[?]) OR (talkgroups.tags && ARRAY[?])) AND (talkgroups.id != ANY(?) AND NOT talkgroups.tags @> ARRAY[?])`,
		f.Talkgroups, f.TalkgroupTagsAny, f.TalkgroupTagsAll, f.TalkgroupsNot, f.TalkgroupTagsNot)

	return FilterQuery{Query: q, Params: args}
}

func PackedTGs(tg []Talkgroup) []int64 {
	s := make([]int64, len(tg))

	for i, v := range tg {
		s[i] = v.Pack()
	}

	return s
}

func (f *Filter) compile() *Filter {
	f.talkgroups = make(map[Talkgroup]bool)
	for _, tg := range f.Talkgroups {
		f.talkgroups[tg] = true
	}

	for _, tg := range f.TalkgroupsNot {
		f.talkgroups[tg] = false
	}

	f.talkgroupTagsAll = make(map[string]bool)
	for _, tag := range f.TalkgroupTagsAll {
		f.talkgroupTagsAll[tag] = true
	}

	f.talkgroupTagsAny = make(map[string]bool)
	for _, tag := range f.TalkgroupTagsAny {
		f.talkgroupTagsAny[tag] = true
	}

	for _, tag := range f.TalkgroupTagsNot {
		f.talkgroupTagsNot[tag] = true
	}

	q := f.filterQuery()
	f.query = &q

	return f
}

func (f *Filter) normalize() {
	tgSort := func(a, b Talkgroup) int {
		if n := cmp.Compare(a.System, b.System); n != 0 {
			return n
		}

		return cmp.Compare(a.Talkgroup, b.Talkgroup)
	}

	slices.SortFunc(f.Talkgroups, tgSort)
	slices.SortFunc(f.TalkgroupsNot, tgSort)
	slices.SortFunc(f.TalkgroupTagsAll, cmp.Compare)
	slices.SortFunc(f.TalkgroupTagsAny, cmp.Compare)
	slices.SortFunc(f.TalkgroupTagsNot, cmp.Compare)
}

func (f *Filter) cacheKey() string {
	f.normalize()
	buf, err := json.Marshal(f)
	if err != nil {
		panic(err)
	}

	return string(buf)
}

func (f *Filter) Test(call *Call) bool {
	return false
}

type FilterCache struct {
	cache map[string]Filter
}

func NewFilterCache() *FilterCache {
	return &FilterCache{
		cache: make(map[string]Filter),
	}
}
