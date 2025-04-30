package radioref_test

import (
	"context"
	"encoding/json"
	"math/rand"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"dynatron.me/x/stillbox/pkg/authz"
	rbacmocks "dynatron.me/x/stillbox/pkg/authz/mocks"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/database/mocks"
	"dynatron.me/x/stillbox/pkg/metrics"
	"dynatron.me/x/stillbox/pkg/talkgroups"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
	"dynatron.me/x/stillbox/pkg/talkgroups/xport"
	"dynatron.me/x/stillbox/pkg/users"
)

func getFixture(fixture string) []byte {
	fixt, err := os.ReadFile("testdata/" + fixture)
	if err != nil {
		panic(err)
	}

	return fixt
}

func TestRadioRef(t *testing.T) {
	// this is for deterministic UUIDs
	uuid.SetRand(rand.New(rand.NewSource(1)))

	tests := []struct {
		name      string
		input     []byte
		impType   string
		sysID     int
		sysName   string
		jsExpect  []byte
		expectErr error
	}{
		{
			name:     "radioreference import",
			impType:  "radioreference",
			input:    getFixture("riscon.txt"),
			jsExpect: getFixture("riscon.json"),
			sysID:    197,
			sysName:  "RISCON",
		},
	}

	subject := users.User{IsAdmin: true}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dbMock := mocks.NewStore(t)
			rbacMock := rbacmocks.NewRBAC(t)
			rbacMock.EXPECT().Check(mock.AnythingOfType("*context.valueCtx"), authz.UseResource("Talkgroup"), mock.AnythingOfType("authz.CheckOption")).Return(&subject, nil)
			if tc.expectErr == nil {
				dbMock.EXPECT().GetSystemName(mock.AnythingOfType("*context.valueCtx"), tc.sysID).Return(tc.sysName, nil)
			}
			ctx := database.CtxWithDB(context.Background(), dbMock)
			ctx = authz.CtxWithRBAC(ctx, rbacMock)
			ctx = tgstore.CtxWithStore(ctx, tgstore.NewCache(dbMock, metrics.NewNoOp()))
			ij := &xport.ImportJob{
				Type:     xport.Format(tc.impType),
				SystemID: tc.sysID,
				Body:     string(tc.input),
			}

			tgs, err := ij.Import(ctx)

			if tc.expectErr != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectErr.Error())
			} else {
				require.NoError(t, err)

				var fixt []talkgroups.Talkgroup
				err = json.Unmarshal(tc.jsExpect, &fixt)
				// jse, _ := json.Marshal(tgs); os.WriteFile("testdata/riscon.json", jse, 0600)
				require.NoError(t, err)

				assert.Equal(t, fixt, tgs)
			}

		})
	}
}
