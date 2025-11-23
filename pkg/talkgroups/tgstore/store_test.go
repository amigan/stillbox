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

func TestTGs(t *testing.T) {
	s := SetupTest()
	defer s.TearDownTest()

	tests := []struct {
		desc        string
		ids         tgsp.IDs
		opts        []tgstore.Option
		expectErr   error
		assertFunc  func(t *testing.T, tgs []*tgsp.Talkgroup)
		assertAlpha []string
		assertLen   *int
	}{
		{
			desc:      "all tgs",
			assertLen: common.PtrTo(296),
		},
		{
			desc:        "single tg",
			ids:         tids("407:10101"),
			assertAlpha: []string{"PFD DISPATCH"},
		},
		{
			desc:        "two tgs",
			ids:         tids("407:1001", "407:10101"),
			assertAlpha: []string{"Narrag PD 1", "PFD DISPATCH"}, // sorted
		},
		{
			desc: "paginated",
			opts: []tgstore.Option{
				tgstore.WithPagination(
					&tgstore.Pagination{
						Pagination: common.Pagination{
							Page: common.PtrTo(4),
						},
					}, 2, nil),
			},
			assertAlpha: []string{"Wide Area 6", "EMA-1"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			st, ctx := s.makeStore(t)

			tgs, err := st.TGs(ctx, tc.ids, tc.opts...)
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
		})
	}
}
