package callstore

import (
	"context"
	"net/url"
	"testing"

	"dynatron.me/x/stillbox/pkg/authz"
	rbacmock "dynatron.me/x/stillbox/pkg/authz/mocks"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/config"
	dbmock "dynatron.me/x/stillbox/pkg/database/mocks"
	"dynatron.me/x/stillbox/pkg/metrics"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type testSub struct{}
func (*testSub) String() string { return "test" }
func (*testSub) GetName() string { return "test" }
func (*testSub) GetRoles() []string { return []string{"admin"} }

func fillCtx(t *testing.T, ctx context.Context) context.Context {
	rm := rbacmock.NewRBAC(t)

	rm.EXPECT().Check(mock.AnythingOfType("*context.cancelCtx"), mock.Anything, mock.Anything).Return(new(testSub), nil)
	return authz.CtxWithRBAC(ctx, rm)
}

var backendCfg []config.CallStorage = []config.CallStorage{
	{
		Name: "b1",
		Backend: "test",
	},
}

type mockAudioBackend struct {
	calls map[string]*calls.CallAudio
}

func (m *mockAudioBackend) Store(ctx context.Context, ca *calls.CallAudio) (AudioRef, error) {
	m.calls[ca.ID.String()] = ca
	return ca.ID.String(), nil
}

func (m *mockAudioBackend) Get(ctx context.Context, call *calls.CallAudio, audioRef AudioRef, opts *CallAudioOptions) (blob []byte, audioURL *url.URL, err error) {
	*call = *m.calls[audioRef.(string)]
	return call.AudioBlob, call.AudioURL, nil
}

func (m *mockAudioBackend) Delete(ctx context.Context, audioRef AudioRef) error {
	delete(m.calls, audioRef.(string))
	return nil
}

func (m *mockAudioBackend) DeleteBulk(ctx context.Context, refs []AudioRef) error {
	return nil
}

func (m *mockAudioBackend) Type() string {
	return "test"
}

func newMockAudioBackend(_ config.ConfigMap) (AudioBackend, error) {
	return &mockAudioBackend{
		calls: map[string]*calls.CallAudio{},
	}, nil
}

func TestMove(t *testing.T) {
	tests := []struct{
		desc string
		par MoveCallParams
		expectErr error
		expectNumRows int
		canceler func(cancel func())
	}{
		{
			desc: "base",
			par: MoveCallParams{
			},
		},
	}

	ctx := fillCtx(t, context.Background())

	registerAudioBackend("test", newMockAudioBackend)

	for _, tc := range tests {
		ctx, cancel := context.WithCancel(ctx)
		db := dbmock.NewStore(t)
		met := metrics.NewNoOp()
		tgc := tgstore.NewCache(db, met)
		st, err := NewStore(ctx, db, tgc, met, backendCfg)
		require.NoError(t, err)
		t.Run(tc.desc, func(t *testing.T) {
			if tc.canceler != nil {
				go tc.canceler(cancel)
			}
			nr, err := st.MoveCallAudio(ctx, tc.par)
			if tc.expectErr != nil {
				assert.ErrorIs(t, err, tc.expectErr)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.expectNumRows, nr)
		})

	}
}
