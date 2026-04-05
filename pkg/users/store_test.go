//go:build integration

package users_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/testutil"
	"dynatron.me/x/stillbox/pkg/authz"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/authz/policy"
	"dynatron.me/x/stillbox/pkg/database"
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

type subjFunc func(context.Context, users.Store) entities.Subject

func (suite *TestSuite) makeStore(t *testing.T, subjF subjFunc) (users.Store, context.Context) {
	rbac, err := authz.New(policy.Policy)
	require.NoError(t, err)

	store := users.NewStore(suite.db)

	var subject entities.Subject = &users.User{}

	ctx := authz.CtxWithRBAC(t.Context(), rbac)

	if subjF != nil {
		subject = subjF(ctx, store)
	}

	return store, entities.CtxWithSubject(ctx, subject)
}

func constraint(c string) error {
	return database.Constraints[c].Error()
}

func usr(name string) subjFunc {
	return func(ctx context.Context, st users.Store) entities.Subject {
		dbu, err := st.GetUser(ctx, name)
		if err != nil {
			panic(err)
		}

		return dbu
	}
}

func TestCreateUser(t *testing.T) {
	s := SetupTest()
	defer s.TearDownTest()

	tests := []struct {
		desc      string
		usr       users.User
		subject   subjFunc
		expectErr error
	}{
		{
			desc: "base",
			usr: users.User{
				Username: "user1",
				Password: "somepass",
				RealName: common.PtrTo("Joe User"),
				Email:    "example@example.com",
				Roles: []string{
					entities.RoleUser,
				},
			},
			subject: usr("admin"),
		},
		{
			desc: "dupe email",
			usr: users.User{
				Username: "user2",
				Password: "somepass",
				RealName: common.PtrTo("Joe User"),
				Email:    "example@example.com",
				Roles: []string{
					entities.RoleUser,
				},
			},
			subject:   usr("admin"),
			expectErr: constraint(database.EmailConstraintName),
		},
		{
			desc: "dupe user",
			usr: users.User{
				Username: "user1",
				Password: "somepass",
				RealName: common.PtrTo("Joe User"),
				Email:    "someone@example.com",
				Roles: []string{
					entities.RoleUser,
				},
			},
			subject:   usr("admin"),
			expectErr: constraint(database.UsernameConstraintName),
		},
		{
			desc: "bad perms",
			usr: users.User{
				Username: "user3",
				Password: "somepass",
				RealName: common.PtrTo("Joe User"),
				Email:    "someone2@example.com",
				Roles: []string{
					entities.RoleUser,
				},
			},
			subject:   usr("user"),
			expectErr: errors.New(`access denied for Action: "create" on Resource: "User"`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			st, ctx := s.makeStore(t, tc.subject)

			_, err := st.AddUser(ctx, &tc.usr)
			if tc.expectErr != nil {
				assert.ErrorContains(t, err, tc.expectErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetUser(t *testing.T) {
	s := SetupTest()
	defer s.TearDownTest()

	tests := []struct {
		desc      string
		username  string
		masked    bool
		subject   subjFunc
		expectErr error
	}{
		{
			desc:     "base",
			username: "submitter",
			subject:  usr("admin"),
		},
		{
			desc:     "masked",
			username: "admin",
			subject:  usr("user"),
			masked:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			st, ctx := s.makeStore(t, tc.subject)

			usr, err := st.GetUserPrivCheck(ctx, tc.username)
			if tc.expectErr != nil {
				assert.ErrorContains(t, err, tc.expectErr.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.username, usr.Username)
				if !tc.masked {
					assert.True(t, strings.HasPrefix(usr.Password, "$"), "password hash '%s' does not start with $", usr.Password)
					assert.Greater(t, len(usr.Password), 5, "hash length < 5")
				} else {
					assert.Empty(t, usr.Password)
					assert.Empty(t, usr.Email)
					assert.Empty(t, usr.PasswordSetAt)
					assert.Nil(t, usr.Roles)
					assert.Nil(t, usr.LastLoginFrom)
				}
			}
		})
	}

}
