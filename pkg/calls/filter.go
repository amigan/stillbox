package calls

type filterQuery struct {
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
	Talkgroups       []Talkgroup `json:"talkgroups"`
	TalkgroupsNot    []Talkgroup `json:"talkgroupsNot"`
	TalkgroupTagsAll []string    `json:"talkgroupTagsAll"`
	TalkgroupTagsAny []string    `json:"talkgroupTagsAny"`
	TalkgroupTagsNot []string    `json:"talkgroupTagsNot"`

	talkgroups       map[Talkgroup]bool
	talkgroupTagsAll map[string]bool
	talkgroupTagsAny map[string]bool
	talkgroupTagsNot map[string]bool

	query *filterQuery
}

func queryParams(s string, p ...any) (string, []any) {
	return s, p
}

func (f *Filter) filterQuery() filterQuery {
	var q string
	var args []interface{}

	q, args = queryParams(
		`((talkgroups.id = ANY(?) OR talkgroups.tags @> ARRAY[?]) OR (talkgroups.tags && ARRAY[?])) AND (talkgroups.id != ANY(?) AND NOT talkgroups.tags @> ARRAY[?])`,
		f.Talkgroups, f.TalkgroupTagsAny, f.TalkgroupTagsAll, f.TalkgroupsNot, f.TalkgroupTagsNot)

	return filterQuery{Query: q, Params: args}
}

func (f *Filter) Packed(tg []Talkgroup) []int64 {
	s := make([]int64, len(f.Talkgroups))

	for i, v := range tg {
		s[i] = v.Pack()
	}

	return s
}

func (f *Filter) Compile() *Filter {
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

type FilterCache struct {
	cache map[string]Filter
}
