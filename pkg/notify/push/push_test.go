//go:build integration

package push_test

import (
	"bytes"
	"context"
	"net/url"
	"strings"
	"testing"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/testutil"
	"dynatron.me/x/stillbox/pkg/alerting/alert"
	"dynatron.me/x/stillbox/pkg/authz"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/authz/policy"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/notify/push"
	"dynatron.me/x/stillbox/pkg/settings"
	"dynatron.me/x/stillbox/pkg/talkgroups"
	"dynatron.me/x/stillbox/pkg/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestSuite struct {
	db *testutil.DB
}

func SetupTest() *TestSuite {
	suite := &TestSuite{
		db: testutil.NewDB(),
	}

	return suite
}

func (suite *TestSuite) TearDownTest() {
	suite.db.Cleanup()
}

type testSender struct {
	t *testing.T
	m map[string]int // map of URL hostname to counter
}

func newTestSender(t *testing.T) *testSender {
	return &testSender{t: t, m: make(map[string]int)}
}

func (ts *testSender) SendAlertToSubscribers(ctx context.Context, subs []database.GetWebPushSubscriptionsSubscribedRow, _ *alert.RenderedAlert) error {
	for _, rawSub := range subs {
		sub, err := push.ReadWebPushSubscription(bytes.NewReader(rawSub.Subscription))
		require.NoError(ts.t, err)

		u, err := url.Parse(sub.Endpoint)
		require.NoError(ts.t, err)
		ts.m[u.Host]++
	}

	return nil
}

func (suite *TestSuite) makePushNotifier(t *testing.T, sender *testSender) (push.PushNotifier, context.Context) {
	ctx := t.Context()
	rbac, err := authz.New(policy.Policy)
	require.NoError(t, err)

	setStore := settings.New(suite.db, settings.ConfigDefaults)

	ctx = authz.CtxWithRBAC(ctx, rbac)
	svcCtx := entities.CtxWithServiceSubject(ctx, "notifiertest")

	n, err := push.NewPushNotifier(svcCtx, &url.URL{Host: "asdfg", Scheme: "https"}, suite.db, rbac, setStore, push.WithSender(sender))
	require.NoError(t, err)

	return n, ctx
}

func makeRA(tgs []string) *alert.RenderedAlertBatch {
	rab := &alert.RenderedAlertBatch{}

	for _, tg := range tgs {
		id := talkgroups.ID{}
		_ = id.UnmarshalText([]byte(tg))
		rab.Alerts = append(rab.Alerts, alert.RenderedAlert{
			Alert: &alert.Alert{
				Base: alert.Base{
					TGID: id,
				},
			},
		})
	}

	return rab
}

func TestSubscriptions(t *testing.T) {
	s := SetupTest()
	defer s.TearDownTest()

	tests := []struct {
		desc      string
		tgs       string
		counts    map[string]int
		expectErr error
	}{
		{
			desc: "base",
			tgs:  "407:2,407:3",
			counts: map[string]int{
				"user1.com": 2,
				"user3.com": 1,
			},
		},
		{
			desc: "base two",
			tgs:  "407:5",
			counts: map[string]int{
				"user3.com": 1,
			},
		},
		{
			desc: "base three",
			tgs:  "407:15192",
			counts: map[string]int{
				"user2.com":     1,
				"alt.user2.com": 1,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			sender := newTestSender(t)
			st, ctx := s.makePushNotifier(t, sender)
			ctx = entities.CtxWithServiceSubject(ctx, "notifiertest")
			err := st.DispatchAlerts(ctx, makeRA(strings.Split(tc.tgs, ",")))
			if tc.expectErr != nil {
				assert.Contains(t, err.Error(), tc.expectErr.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.counts, sender.m)
		})
	}
}

func tgids(s ...string) talkgroups.IDs {
	return talkgroups.TGIDs(s...)
}

func TestSubscribe(t *testing.T) {
	s := SetupTest()
	defer s.TearDownTest()

	sender := newTestSender(t)
	st, ctx := s.makePushNotifier(t, sender)

	tests := []struct {
		desc           string
		uid            int
		subs           *push.SubscriptionSet
		unsubs         *push.SubscriptionSet
		expectSubs     *push.SubscriptionSet
		expectSubErr   error
		expectUnsubErr error
	}{
		{
			desc: "base",
			uid:  1,
			subs: &push.SubscriptionSet{
				Talkgroups: tgids("407:168"),
				Systems:    []int32{3348},
			},
			expectSubs: &push.SubscriptionSet{
				Talkgroups: tgids("407:2", "407:3", "407:168"),
				Systems:    []int32{3348},
			},
		},
		{
			desc: "unsub",
			uid:  1,
			unsubs: &push.SubscriptionSet{
				Talkgroups: tgids("407:168"),
			},
			expectSubs: &push.SubscriptionSet{
				Talkgroups: tgids("407:2", "407:3"),
				Systems:    []int32{3348},
			},
		},
		{
			desc: "unsub sys",
			uid:  1,
			unsubs: &push.SubscriptionSet{
				Systems: []int32{3348},
			},
			expectSubs: &push.SubscriptionSet{
				Talkgroups: tgids("407:2", "407:3"),
			},
		},
		{
			desc: "unsub all",
			uid:  1,
			unsubs: &push.SubscriptionSet{
				UnsubscribeAll: common.PtrTo(true),
			},
			expectSubs: &push.SubscriptionSet{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := entities.CtxWithSubject(ctx, &users.User{ID: users.UserID(tc.uid)})
			if tc.subs != nil {
				err := st.Subscribe(ctx, tc.subs)
				if tc.expectSubErr != nil {
					assert.Contains(t, err.Error(), tc.expectSubErr.Error())
				} else {
					assert.NoError(t, err)
				}
			}

			if tc.unsubs != nil {
				err := st.Unsubscribe(ctx, tc.unsubs)
				if tc.expectUnsubErr != nil {
					assert.Contains(t, err.Error(), tc.expectUnsubErr.Error())
				} else {
					assert.NoError(t, err)
				}
			}

			finalSubs, err := st.Subscriptions(ctx)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectSubs, finalSubs)
		})
	}
}
