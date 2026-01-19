//go:build integration

package tgstore_test

import (
	"context"
	"testing"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/testutil"
	"dynatron.me/x/stillbox/pkg/authz"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/authz/policy"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/metrics"
	tgsp "dynatron.me/x/stillbox/pkg/talkgroups"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
	"dynatron.me/x/stillbox/pkg/users"
	"github.com/google/uuid"
	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	suite.Suite
	db *testutil.DB
}

type testHook func(context.Context, *testing.T, tgstore.Store)

func tid(s string) tgsp.ID {
	var tg tgsp.ID
	err := tg.UnmarshalText([]byte(s))
	if err != nil {
		panic(err)
	}

	return tg
}

func tids(ids ...string) []tgsp.ID {
	r := make([]tgsp.ID, 0, len(ids))

	for _, s := range ids {
		r = append(r, tid(s))
	}

	return r
}

func SetupTest() *TestSuite {
	suite := &TestSuite{
		db: testutil.NewDB(testutil.DailyPartConfig()),
	}

	return suite
}

func (suite *TestSuite) TearDownTest() {
	suite.db.Cleanup()
}

func (suite *TestSuite) makeStore(t *testing.T, subject entities.Subject) (tgstore.Store, context.Context) {
	rbac, err := authz.New(policy.Policy)
	require.NoError(t, err)

	if subject == nil {
		subject = &users.User{}
	}

	ctx := authz.CtxWithRBAC(t.Context(), rbac)
	ctx = entities.CtxWithSubject(ctx, subject)
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
		subject   entities.Subject
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
		{
			desc:     "forbidden",
			systemID: 407,
			subject:  &entities.PublicSubject{},
			assert: tgsAssertion{
				expectErr: authz.ErrAccessDenied,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			st, ctx := s.makeStore(t, tc.subject)

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
		desc     string
		ids      tgsp.IDs
		opts     []tgstore.Option
		assert   tgsAssertion
		preFunc  testHook
		postFunc testHook
		subject  entities.Subject
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
			postFunc: func(_ context.Context, t *testing.T, _ tgstore.Store) {
				assert.Equal(t, 298, totalDest)
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
		{
			desc: "mixed cached",
			preFunc: func(ctx context.Context, t *testing.T, st tgstore.Store) {
				err := st.Hint(ctx, tids("407:10101", "3348:153"))
				require.NoError(t, err)
			},
			ids: tids("407:10101", "3348:153", "407:1869", "407:11186", "407:11002"),
			postFunc: func(_ context.Context, t *testing.T, tgs tgstore.Store) {
				assert.Equal(t, promtestutil.ToFloat64(tgs.Metrics().Hits), 2.0)
				assert.Equal(t, promtestutil.ToFloat64(tgs.Metrics().Misses), 3.0)
			},
		},
		{
			desc:    "forbidden",
			ids:     tids("407:10101", "3348:153", "407:1869", "407:11186", "407:11002"),
			subject: &entities.PublicSubject{},
			assert: tgsAssertion{
				expectErr: authz.ErrAccessDenied,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			st, ctx := s.makeStore(t, tc.subject)

			if tc.preFunc != nil {
				tc.preFunc(ctx, t, st)
			}

			tgs, err := st.TGs(ctx, tc.ids, tc.opts...)
			tc.assert.assert(t, tgs, err)

			if tc.postFunc != nil {
				tc.postFunc(ctx, t, st)
			}
		})
	}
}

func TestTG(t *testing.T) {
	s := SetupTest()
	defer s.TearDownTest()

	tests := []struct {
		desc     string
		tg       string
		assert   tgsAssertion
		preFunc  testHook
		postFunc testHook
		subject  entities.Subject
	}{
		{
			desc: "base",
			tg:   "407:10101",
			assert: tgsAssertion{
				assertAlpha: []string{"PFD DISPATCH"},
			},
		},
		{
			desc: "noexist",
			tg:   "407:9966",
			assert: tgsAssertion{
				expectErr: tgstore.ErrNotFound,
			},
		},
		{
			desc:    "forbidden",
			tg:      "407:9966",
			subject: &entities.PublicSubject{},
			assert: tgsAssertion{
				expectErr: authz.ErrAccessDenied,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			st, ctx := s.makeStore(t, tc.subject)

			if tc.preFunc != nil {
				tc.preFunc(ctx, t, st)
			}

			tg, err := st.TG(ctx, tids(tc.tg)[0])
			tc.assert.assert(t, []*tgsp.Talkgroup{tg}, err)

			if tc.postFunc != nil {
				tc.postFunc(ctx, t, st)
			}
		})
	}
}

func TestCreateSystem(t *testing.T) {
	s := SetupTest()
	defer s.TearDownTest()

	tests := []struct {
		desc      string
		id        int
		name      string
		learned   bool
		subject   entities.Subject
		expectErr error
	}{
		{
			desc:    "base",
			id:      0xbeef,
			name:    "SomeSys",
			subject: &users.User{Roles: []string{entities.RoleAdmin}},
		},
		{
			desc:      "forbidden submitter",
			id:        0xbeef,
			name:      "SomeSys",
			subject:   &users.User{Roles: []string{entities.RoleSubmitter}},
			expectErr: authz.ErrAccessDenied,
		},
		{
			desc:      "forbidden public",
			id:        0xbeef,
			name:      "SomeSys",
			subject:   &entities.PublicSubject{},
			expectErr: authz.ErrAccessDenied,
		},
		{
			desc:      "forbidden user",
			id:        0xbeef,
			name:      "SomeSys",
			subject:   &users.User{},
			expectErr: authz.ErrAccessDenied,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			st, ctx := s.makeStore(t, tc.subject)

			err := st.CreateSystem(ctx, tc.id, tc.name, tc.learned)
			if tc.expectErr != nil {
				assert.ErrorContains(t, err, tc.expectErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLearnTG(t *testing.T) {
	s := SetupTest()
	defer s.TearDownTest()

	tests := []struct {
		desc      string
		learned   bool
		tg        tgsp.ID
		name      string
		subject   entities.Subject
		expectErr error
	}{
		{
			desc:    "base",
			tg:      tid("407:10998"),
			name:    "North Place Police",
			subject: &users.User{Roles: []string{entities.RoleAdmin}},
		},
		{
			desc:    "allowed submitter",
			tg:      tid("407:10990"),
			name:    "South Place Police",
			subject: &users.User{Roles: []string{entities.RoleSubmitter}},
		},
		{
			desc:      "forbidden public",
			name:      "Place",
			tg:        tid("407:10998"),
			subject:   &entities.PublicSubject{},
			expectErr: authz.ErrAccessDenied,
		},
		{
			desc:      "forbidden user",
			tg:        tid("407:10998"),
			name:      "Place",
			subject:   &users.User{},
			expectErr: authz.ErrAccessDenied,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			st, ctx := s.makeStore(t, tc.subject)

			otg, err := st.LearnTG(ctx, &calls.Call{
				ID:             uuid.New(),
				System:         int(tc.tg.System),
				Talkgroup:      int(tc.tg.Talkgroup),
				TalkgroupLabel: &tc.name,
				TGAlphaTag:     &tc.name,
				TalkgroupGroup: &tc.name,
			})
			if tc.expectErr != nil {
				assert.ErrorContains(t, err, tc.expectErr.Error())
			} else {
				assert.NoError(t, err)
				tg, err := st.TG(ctx, tc.tg)
				require.NoError(t, err)

				assert.Equal(t, tg, otg)
			}
		})
	}
}

func TestUpsertTGs(t *testing.T) {
	s := SetupTest()
	defer s.TearDownTest()

	totalDest := 0

	tests := []struct {
		desc     string
		ids      tgsp.IDs
		opts     []tgstore.Option
		assert   tgsAssertion
		preFunc  testHook
		postFunc testHook
		subject  entities.Subject
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
			postFunc: func(_ context.Context, t *testing.T, _ tgstore.Store) {
				assert.Equal(t, 298, totalDest)
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
		{
			desc: "mixed cached",
			preFunc: func(ctx context.Context, t *testing.T, st tgstore.Store) {
				err := st.Hint(ctx, tids("407:10101", "3348:153"))
				require.NoError(t, err)
			},
			ids: tids("407:10101", "3348:153", "407:1869", "407:11186", "407:11002"),
			postFunc: func(_ context.Context, t *testing.T, tgs tgstore.Store) {
				assert.Equal(t, promtestutil.ToFloat64(tgs.Metrics().Hits), 2.0)
				assert.Equal(t, promtestutil.ToFloat64(tgs.Metrics().Misses), 3.0)
			},
		},
		{
			desc:    "forbidden",
			ids:     tids("407:10101", "3348:153", "407:1869", "407:11186", "407:11002"),
			subject: &entities.PublicSubject{},
			assert: tgsAssertion{
				expectErr: authz.ErrAccessDenied,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			st, ctx := s.makeStore(t, tc.subject)

			if tc.preFunc != nil {
				tc.preFunc(ctx, t, st)
			}

			tgs, err := st.TGs(ctx, tc.ids, tc.opts...)
			tc.assert.assert(t, tgs, err)

			if tc.postFunc != nil {
				tc.postFunc(ctx, t, st)
			}
		})
	}
}
