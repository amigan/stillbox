package rbac_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/incidents"
	"dynatron.me/x/stillbox/pkg/rbac"
	"dynatron.me/x/stillbox/pkg/talkgroups"
	"dynatron.me/x/stillbox/pkg/users"
	"github.com/el-mike/restrict/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRBAC(t *testing.T) {
	tests := []struct {
		name      string
		subject   rbac.Subject
		resource  rbac.Resource
		action    string
		expectErr error
	}{
		{
			name: "admin update talkgroup",
			subject: &users.User{
				ID:      2,
				IsAdmin: true,
			},
			resource:  &talkgroups.Talkgroup{},
			action:    rbac.ActionUpdate,
			expectErr: nil,
		},
		{
			name: "admin update incident",
			subject: &users.User{
				ID:      2,
				IsAdmin: true,
			},
			resource: &incidents.Incident{
				Name:  "test incident",
				Owner: 4,
			},
			action:    rbac.ActionUpdate,
			expectErr: nil,
		},
		{
			name: "user update incident not owner",
			subject: &users.User{
				ID: 2,
			},
			resource: &incidents.Incident{
				Name:  "test incident",
				Owner: 4,
			},
			action:    rbac.ActionUpdate,
			expectErr: errors.New(`access denied for Action: "update" on Resource: "Incident"`),
		},
		{
			name: "user update incident owner",
			subject: &users.User{
				ID: 2,
			},
			resource: &incidents.Incident{
				Name:  "test incident",
				Owner: 2,
			},
			action:    rbac.ActionUpdate,
			expectErr: nil,
		},
		{
			name: "user delete incident not owner",
			subject: &users.User{
				ID: 2,
			},
			resource: &incidents.Incident{
				Name:  "test incident",
				Owner: 6,
			},
			action:    rbac.ActionDelete,
			expectErr: errors.New(`access denied for Action: "delete" on Resource: "Incident"`),
		},
		{
			name: "admin update call",
			subject: &users.User{
				ID:      2,
				IsAdmin: true,
			},
			resource: &calls.Call{
				Submitter: common.PtrTo(users.UserID(4)),
			},
			action:    rbac.ActionUpdate,
			expectErr: nil,
		},
		{
			name: "user update call not owner",
			subject: &users.User{
				ID: 2,
			},
			resource: &calls.Call{
				Submitter: common.PtrTo(users.UserID(4)),
			},
			action:    rbac.ActionUpdate,
			expectErr: errors.New(`access denied for Action: "update" on Resource: "Call"`),
		},
		{
			name: "user update call owner",
			subject: &users.User{
				ID: 2,
			},
			resource: &calls.Call{
				Submitter: common.PtrTo(users.UserID(2)),
			},
			action:    rbac.ActionUpdate,
			expectErr: nil,
		},
		{
			name: "user update call nil submitter",
			subject: &users.User{
				ID: 2,
			},
			resource: &calls.Call{
				Submitter: nil,
			},
			action:    rbac.ActionUpdate,
			expectErr: errors.New(`access denied for Action: "update" on Resource: "Call"`),
		},
		{
			name: "user delete call not owner",
			subject: &users.User{
				ID: 2,
			},
			resource: &calls.Call{
				Submitter: common.PtrTo(users.UserID(6)),
			},
			action:    rbac.ActionDelete,
			expectErr: errors.New(`access denied for Action: "delete" on Resource: "Call"`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := rbac.CtxWithSubject(context.Background(), tc.subject)
			rb, err := rbac.New()
			require.NoError(t, err)
			sub, err := rb.Check(ctx, tc.resource, rbac.WithActions(tc.action))
			if tc.expectErr != nil {
				assert.Equal(t, tc.expectErr.Error(), err.Error())
			} else {
				if !assert.NoError(t, err) {
					accErr(err)
				}
			}
			assert.Equal(t, tc.subject, sub)
		})
	}
}

func accErr(err error) {
	if accessError, ok := err.(*restrict.AccessDeniedError); ok {
		// Error() implementation. Returns a message in a form: "access denied for Action/s: ... on Resource: ..."
		fmt.Println(accessError)
		// Returns an AccessRequest that failed.
		fmt.Println(accessError.Request)
		// Returns first reason for the denied access.
		// Especially helpful in fail-early mode, where there will only be one Reason.
		fmt.Println(accessError.FirstReason())

		// Reasons property will hold all errors that caused the access to be denied.
		for _, permissionErr := range accessError.Reasons {
			fmt.Println(permissionErr)
			fmt.Println(permissionErr.Action)
			fmt.Println(permissionErr.RoleName)
			fmt.Println(permissionErr.ResourceName)

			// Returns first ConditionNotSatisfied error for given PermissionError, if any was returned for given PermissionError.
			// Especially helpful in fail-early mode, where there will only be one failed Condition.
			fmt.Println(permissionErr.FirstConditionError())

			// ConditionErrors property will hold all ConditionNotSatisfied errors.
			for _, conditionErr := range permissionErr.ConditionErrors {
				fmt.Println(conditionErr)
				fmt.Println(conditionErr.Reason)

				// Every ConditionNotSatisfied contains an instance of Condition that returned it,
				// so it can be tested using type assertion to get more details about failed Condition.
				if emptyCondition, ok := conditionErr.Condition.(*restrict.EmptyCondition); ok {
					fmt.Println(emptyCondition.ID)
				}
			}
		}
	}
}
