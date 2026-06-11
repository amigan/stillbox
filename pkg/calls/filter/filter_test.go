package filter_test

import (
	"context"
	"strings"
	"testing"

	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/calls/filter"
	"dynatron.me/x/stillbox/pkg/database"
	dbmocks "dynatron.me/x/stillbox/pkg/database/mocks"
	"dynatron.me/x/stillbox/pkg/talkgroups"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
	tgsmocks "dynatron.me/x/stillbox/pkg/talkgroups/tgstore/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type tvector map[string]bool

func stv(s string) *calls.Call {
	var tid talkgroups.ID
	err := tid.UnmarshalText([]byte(s))
	if err != nil {
		panic(err)
	}

	return &calls.Call{
		System:    int(tid.System),
		Talkgroup: int(tid.Talkgroup),
	}
}

type tagSet map[string]string

func tgids(s ...string) talkgroups.IDs {
	return talkgroups.TGIDs(s...)
}

func stg(s string) []string {
	return strings.Split(s, " ")
}

func (ts tagSet) filter(_ context.Context, all, anyS, not []string) talkgroups.IDs {
	r := make(talkgroups.IDs, 0, len(ts))

	allMap := make(map[string]struct{})
	for _, v := range all {
		allMap[v] = struct{}{}
	}

	anyMap := make(map[string]struct{})
	for _, v := range anyS {
		anyMap[v] = struct{}{}
	}

	notMap := make(map[string]struct{})
	for _, v := range not {
		notMap[v] = struct{}{}
	}

	for k, v := range ts {
		tags := strings.Split(v, ",")

		var res bool
		allCount := 0
		hasNot := false
		hasAny := false
		for _, t := range tags {
			if _, inAll := allMap[t]; inAll {
				allCount++
			}

			_, has := anyMap[t]
			if has {
				hasAny = true
			}

			_, inNot := notMap[t]
			if inNot {
				hasNot = true
			}
		}

		if hasAny {
			res = true
		}

		if len(all) != allCount {
			res = false
		}

		if hasNot {
			res = false
		}

		if res {
			var id talkgroups.ID
			err := id.UnmarshalText([]byte(k))
			if err != nil {
				panic(err)
			}
			r = append(r, id)
		}
	}

	return r
}

func tgsMock(t *testing.T, ts tagSet, noLookup bool) tgstore.Store {
	s := tgsmocks.NewStore(t)

	if !noLookup {
		s.On("TGsByTags", mock.AnythingOfType("*context.valueCtx"),
			mock.AnythingOfType("[]string"), mock.AnythingOfType("[]string"), mock.AnythingOfType("[]string")).
			Return(ts.filter, nil)
	}

	return s
}

func TestFilterCompile(t *testing.T) {
	tests := []struct {
		desc       string
		filter     *filter.Filter
		tgTags     tagSet
		noLookup   bool
		testVector tvector
	}{
		{
			desc: "base case",
			tgTags: tagSet{
				"407:10101": "law,dispatch,providence",
				"407:1372":  "fire,fireground,statewide",
				"407:1736":  "law,statewide,law-talk",
				"407:1796":  "law,statewide,law-talk,tac1",
				"407:1296":  "law,statewide,law-talk,tac2",
				"407:1196":  "law,statewide,law-tac,tac2",
				"407:10001": "law,providence,law-talk,tac2",
			},
			filter: &filter.Filter{
				Talkgroups:       tgids("0x197:10101"),
				TalkgroupsNot:    tgids("407:1657"),
				TalkgroupTagsAll: stg("law statewide"),
				TalkgroupTagsNot: stg("law-talk"),
				TalkgroupTagsAny: stg("tac1 tac2"),
			},
			testVector: tvector{
				"407:10101": true,
				"407:1372":  false,
				"407:1736":  false,
				"407:1796":  false,
				"407:1196":  true,
				"407:1296":  false,
				"407:1657":  false,
				"407:1658":  false,
				"407:10001": false,
			},
		},
		{
			desc: "all case",
			tgTags: tagSet{
				"407:10101": "law,dispatch,providence",
				"407:1372":  "fire,fireground,statewide",
				"407:1736":  "law,statewide,law-talk",
				"407:1796":  "law,statewide,law-talk,tac1",
				"407:1296":  "law,statewide,law-talk,tac2",
				"407:1196":  "law,statewide,law-tac,tac2",
				"407:10001": "law,providence,law-talk,tac2",
			},
			filter: &filter.Filter{
				All: true,
			},
			noLookup: true,
			testVector: tvector{
				"407:10101": true,
				"407:1372":  true,
				"407:1736":  true,
				"407:1796":  true,
				"407:1196":  true,
				"407:1296":  true,
				"407:1657":  true,
				"407:1658":  true,
				"407:10001": true,
			},
		},
		{
			desc: "nil case",
			tgTags: tagSet{
				"407:10101": "law,dispatch,providence",
				"407:1372":  "fire,fireground,statewide",
				"407:1736":  "law,statewide,law-talk",
				"407:1796":  "law,statewide,law-talk,tac1",
				"407:1296":  "law,statewide,law-talk,tac2",
				"407:1196":  "law,statewide,law-tac,tac2",
				"407:10001": "law,providence,law-talk,tac2",
			},
			filter:   nil,
			noLookup: true,
			testVector: tvector{
				"407:10101": true,
				"407:1372":  true,
				"407:1736":  true,
				"407:1796":  true,
				"407:1196":  true,
				"407:1296":  true,
				"407:1657":  true,
				"407:1658":  true,
				"407:10001": true,
			},
		},
	}

	dbMock := dbmocks.NewStore(t)

	ctx := database.CtxWithDB(context.Background(), dbMock)

	for _, tc := range tests {
		tgS := tgsMock(t, tc.tgTags, tc.noLookup)
		ctx = tgstore.CtxWithStore(ctx, tgS)
		t.Run(tc.desc, func(t *testing.T) {
			for tgS, expectedResult := range tc.testVector {
				r := tc.filter.Test(ctx, stv(tgS))
				assert.Equal(t, expectedResult, r, tgS)
			}
		})
	}
}
