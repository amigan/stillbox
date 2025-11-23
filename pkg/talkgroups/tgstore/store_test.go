//go:build integration

package tgstore_test

import (
	"context"
	"testing"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/testutil"
	"dynatron.me/x/stillbox/pkg/authz"
	authzMocks "dynatron.me/x/stillbox/pkg/authz/mocks"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/metrics"
	tgsp "dynatron.me/x/stillbox/pkg/talkgroups"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	suite.Suite
	db testutil.DB
}

func tids(ids ...string) []tgsp.ID {
	r := make([]tgsp.ID, 0, len(ids))

	for _, s := range ids {
		var tg tgsp.ID
		err := tg.UnmarshalText([]byte(s))
		if err != nil {
			panic(err)
		}

		r = append(r, tg)
	}

	return r
}

func SetupTest() *TestSuite {
	suite := new(TestSuite)
	if suite.db.Postgres != nil {
		suite.db.Close()
	}

	suite.db = testutil.NewDB(config.Partition{
		Enabled:  true,
		Schema:   suite.db.SchemaName,
		Interval: "daily",
	})

	return suite
}

func (suite *TestSuite) TearDownTest() {
	suite.db.Cleanup()
}

func (suite *TestSuite) makeStore(t *testing.T) (tgstore.Store, context.Context) {
	rbacMock := authzMocks.NewRBAC(t)
	rbacMock.EXPECT().Check(mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	ctx := authz.CtxWithRBAC(t.Context(), rbacMock)
	metrics, _ := metrics.NewMetrics(config.Metrics{})

	return tgstore.NewCache(suite.db, metrics), ctx
}

type tgsAssertion struct {
	assertFunc  func(t *testing.T, tgs []*tgsp.Talkgroup)
	assertAlpha []string
	assertLen   *int
	expectErr   error
}

func (tc tgsAssertion) assert(t *testing.T, tgs []*tgsp.Talkgroup, err error) {
	if tc.expectErr != nil {
		assert.ErrorContains(t, err, tc.expectErr.Error())
	} else {
		assert.NoError(t, err)
		if tc.assertFunc != nil {
			tc.assertFunc(t, tgs)
		}

		if tc.assertAlpha != nil {
			ats := make([]string, 0, len(tgs))
			for _, tg := range tgs {
				if tg.AlphaTag != nil {
					ats = append(ats, *tg.AlphaTag)
				}
			}

			assert.Equal(t, tc.assertAlpha, ats)
		}

		if tc.assertLen != nil {
			assert.Len(t, tgs, *tc.assertLen)
		}
	}
}

func TestSystemTGs(t *testing.T) {
	s := SetupTest()
	defer s.TearDownTest()

	totalDest := 0

	tests := []struct {
		desc      string
		systemID  int
		opts      []tgstore.Option
		expectErr error
		assert    tgsAssertion
	}{
		{
			desc:     "all tgs",
			systemID: 407,
			assert: tgsAssertion{
				assertLen: common.PtrTo(296),
			},
		},
		{
			desc:     "all tgs 2",
			systemID: 3348,
			assert: tgsAssertion{
				assertLen: common.PtrTo(2),
			},
		},
		{
			desc:     "paginated",
			systemID: 407,
			opts: []tgstore.Option{
				tgstore.WithPagination(
					&tgstore.Pagination{
						Pagination: common.Pagination{
							Page: common.PtrTo(4),
						},
					}, 2, &totalDest),
			},
			assert: tgsAssertion{
				assertAlpha: []string{"Wide Area 6", "EMA-1"},
			},
		},
		{
			desc:     "filtered",
			systemID: 3348,
			opts: []tgstore.Option{
				tgstore.WithFilter(common.PtrTo("MBTA")),
			},
			assert: tgsAssertion{
				assertLen: common.PtrTo(1),
			},
		},
		{
			desc:     "paginated filtered",
			systemID: 407,
			opts: []tgstore.Option{
				tgstore.WithPagination(
					&tgstore.Pagination{
						Pagination: common.Pagination{
							Page: common.PtrTo(4),
						},
					}, 2, nil),
				tgstore.WithFilter(common.PtrTo("Fire")),
			},
			assert: tgsAssertion{
				assertAlpha: []string{"Narrag FDFG2", "Narrag EMS"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			st, ctx := s.makeStore(t)

			// test case sanity check
			require.NotZero(t, tc.systemID)

			tgs, err := st.SystemTGs(ctx, tc.systemID, tc.opts...)
			tc.assert.assert(t, tgs, err)

			// hacky
			if tc.desc == "paginated" {
				assert.Equal(t, 296, totalDest)
			}
		})
	}
}

func TestTGs(t *testing.T) {
	s := SetupTest()
	defer s.TearDownTest()

	totalDest := 0

	tests := []struct {
		desc   string
		ids    tgsp.IDs
		opts   []tgstore.Option
		assert tgsAssertion
	}{
		{
			desc: "all tgs",
			assert: tgsAssertion{
				assertLen: common.PtrTo(298),
			},
		},
		{
			desc: "single tg",
			ids:  tids("407:10101"),
			assert: tgsAssertion{
				assertAlpha: []string{"PFD DISPATCH"},
			},
		},
		{
			desc: "two tgs",
			ids:  tids("407:1001", "407:10101"),
			assert: tgsAssertion{
				assertAlpha: []string{"Narrag PD 1", "PFD DISPATCH"}, // sorted
			},
		},
		{
			desc: "paginated",
			opts: []tgstore.Option{
				tgstore.WithPagination(
					&tgstore.Pagination{
						Pagination: common.Pagination{
							Page: common.PtrTo(4),
						},
					}, 2, &totalDest),
			},
			assert: tgsAssertion{
				assertAlpha: []string{"Wide Area 6", "EMA-1"},
			},
		},
		{
			desc: "filtered",
			opts: []tgstore.Option{
				tgstore.WithFilter(common.PtrTo("Fire")),
			},
			assert: tgsAssertion{
				assertLen: common.PtrTo(99),
			},
		},
		{
			desc: "paginated filtered",
			opts: []tgstore.Option{
				tgstore.WithPagination(
					&tgstore.Pagination{
						Pagination: common.Pagination{
							Page: common.PtrTo(4),
						},
					}, 2, nil),
				tgstore.WithFilter(common.PtrTo("Fire")),
			},
			assert: tgsAssertion{
				assertAlpha: []string{"Narrag FDFG2", "Narrag EMS"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			st, ctx := s.makeStore(t)

			tgs, err := st.TGs(ctx, tc.ids, tc.opts...)
			tc.assert.assert(t, tgs, err)

			// hacky
			if tc.desc == "paginated" {
				assert.Equal(t, 298, totalDest)
			}
		})
	}
}
