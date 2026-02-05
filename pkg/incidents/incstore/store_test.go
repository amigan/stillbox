//go:build integration

package incstore_test

import (
	"context"
	"errors"
	"testing"

	"dynatron.me/x/stillbox/internal/testutil"
	"dynatron.me/x/stillbox/pkg/authz"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/authz/policy"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/incidents"
	"dynatron.me/x/stillbox/pkg/incidents/incstore"
	"dynatron.me/x/stillbox/pkg/users"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	suite.Suite
	db *testutil.DB
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

func (suite *TestSuite) makeStore(t *testing.T, subject entities.Subject) (incstore.Store, context.Context) {
	rbac, err := authz.New(policy.Policy)
	require.NoError(t, err)

	if subject == nil {
		subject = &users.User{}
	}

	ctx := authz.CtxWithRBAC(t.Context(), rbac)
	ctx = entities.CtxWithSubject(ctx, subject)

	return incstore.NewStore(suite.db), ctx
}

func TestCreateIncident(t *testing.T) {
	s := SetupTest()
	defer s.TearDownTest()

	tests := []struct {
		desc      string
		inc       incidents.Incident
		subject   entities.Subject
		expectErr error
	}{
		{
			desc: "base",
			inc: incidents.Incident{
				OwnerID: 1,
				Name:    "test1",
			},
			subject: &users.User{ID: 1, Roles: []string{entities.RoleAdmin}},
		},
		{
			desc: "base user",
			inc: incidents.Incident{
				OwnerID: 2,
				Name:    "test2",
			},
			subject: &users.User{ID: 2},
		},
		{
			desc: "nxcall",
			inc: incidents.Incident{
				OwnerID: 1,
				Name:    "test3",
				Calls: []incidents.IncidentCall{
					{
						Call: calls.Call{
							ID: uuid.MustParse(testutil.UUID("nxcall")),
						},
					},
				},
			},
			subject: &users.User{ID: 2},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			st, ctx := s.makeStore(t, tc.subject)

			_, err := st.CreateIncident(ctx, tc.inc)
			if tc.expectErr != nil {
				assert.ErrorContains(t, err, tc.expectErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetIncident(t *testing.T) {
	s := SetupTest()
	defer s.TearDownTest()

	tests := []struct {
		desc          string
		incID         string
		subject       entities.Subject
		opts          []incstore.IncidentOption
		expectIncName string
		expectErr     error
		expectCalls   []string
	}{
		{
			desc:          "base",
			incID:         testutil.UUID("inc1"),
			subject:       &users.User{ID: 1, Roles: []string{entities.RoleAdmin}},
			expectIncName: "Dumpster Fire",
			expectCalls:   []string{testutil.UUID("call1")},
		},
		{
			desc:          "without calls",
			incID:         testutil.UUID("inc1"),
			subject:       &users.User{ID: 1, Roles: []string{entities.RoleAdmin}},
			opts:          []incstore.IncidentOption{incstore.WithoutCalls()},
			expectIncName: "Dumpster Fire",
		},
		{
			desc:          "denied",
			incID:         testutil.UUID("inc1"),
			subject:       &entities.PublicSubject{},
			expectIncName: "Dumpster Fire",
			expectErr:     errors.New(`access denied for Action: "read" on Resource: "Incident"`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			st, ctx := s.makeStore(t, tc.subject)

			incID := uuid.MustParse(tc.incID)
			inc, err := st.Incident(ctx, incID, tc.opts...)
			if tc.expectErr != nil {
				assert.ErrorContains(t, err, tc.expectErr.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectIncName, inc.Name)
				if tc.expectCalls != nil {
					callIDs := make([]string, 0, len(inc.Calls))
					for _, c := range inc.Calls {
						callIDs = append(callIDs, c.ID.String())
					}
					assert.ElementsMatch(t, tc.expectCalls, callIDs)
				} else {
					assert.Nil(t, inc.Calls)
				}
			}
		})
	}

}

func TestGetIncidentCalls(t *testing.T) {
	s := SetupTest()
	defer s.TearDownTest()

	tests := []struct {
		desc             string
		incID            string
		subject          entities.Subject
		expectErr        error
		expectCallsCount int
	}{
		{
			desc:             "base",
			incID:            testutil.UUID("inc1"),
			subject:          &users.User{ID: 1, Roles: []string{entities.RoleAdmin}},
			expectCallsCount: 1,
		},
		{
			desc:      "denied",
			incID:     testutil.UUID("inc1"),
			subject:   &entities.PublicSubject{},
			expectErr: errors.New(`access denied for Action: "read" on Resource: "Incident"`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			st, ctx := s.makeStore(t, tc.subject)

			incID := uuid.MustParse(tc.incID)
			inc, err := st.IncidentCalls(ctx, incID, &incstore.CallsFilter{})
			if tc.expectErr != nil {
				assert.ErrorContains(t, err, tc.expectErr.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectCallsCount, inc.Count)
				assert.Len(t, inc.Calls, tc.expectCallsCount)
			}
		})
	}

}
