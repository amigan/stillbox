package callstore_test

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"testing"
	"time"

	"dynatron.me/x/stillbox/internal/testutil"
	"dynatron.me/x/stillbox/pkg/authz"
	rbacmock "dynatron.me/x/stillbox/pkg/authz/mocks"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/calls/callstore"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/database"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

func fillCtxRbac(t *testing.T, ctx context.Context) context.Context {
	rm := rbacmock.NewRBAC(t)

	rm.EXPECT().Check(mock.Anything, mock.Anything, mock.Anything).Return(new(testutil.AdminSubject), nil)
	return authz.CtxWithRBAC(ctx, rm)
}

type mockAudioBackend struct {
	calls map[string]*calls.CallAudio
	st    callstore.Store
}

func (m *mockAudioBackend) Store(ctx context.Context, ca *calls.CallAudio) (callstore.AudioRef, error) {
	m.calls[ca.ID.String()] = ca
	return callstore.AbsoluteRef(ca.ID.String()), nil
}

func (m *mockAudioBackend) Get(ctx context.Context, call *calls.CallAudio, audioRef callstore.AudioRef, opts *callstore.CallAudioOptions) (blob []byte, audioURL *url.URL, err error) {
	*call = *m.calls[audioRef.String()]
	return call.AudioBlob, call.AudioURL, nil
}

func (m *mockAudioBackend) Delete(ctx context.Context, _ *calls.CallAudio, audioRef callstore.AudioRef) error {
	delete(m.calls, audioRef.String())
	return nil
}

func (m *mockAudioBackend) DeleteBulk(ctx context.Context, refs []callstore.AbsoluteRef) error {
	return nil
}

func (m *mockAudioBackend) Type() string {
	return "test"
}

func (m *mockAudioBackend) Prune(ctx context.Context, audioRef string, pruneAfter *time.Time) (newPruneAfter *time.Time, err error) {
	return nil, nil
}

func (m *mockAudioBackend) makeCalls(ctx context.Context, n int) []database.GetCallAudioRow {
	rows := make([]database.GetCallAudioRow, n)

	for i := range n {
		id := uuid.New()
		ref := fmt.Sprintf(`{"test":"%s"}`, id.String())
		rows[i] = database.GetCallAudioRow{
			ID:       id,
			AudioRef: []byte(ref),
		}
		_, _ = m.Store(ctx, callstore.GetCallAudioRowToSkinnyCallAudio(&rows[i]))
	}

	return rows
}

func newMockAudioBackend(st callstore.Store, _ config.ConfigMap) (callstore.AudioBackend, error) {
	backendMake.Do(func() {
		mbe = &mockAudioBackend{
			calls: map[string]*calls.CallAudio{},
			st:    st,
		}
		dbCalls = mbe.makeCalls(context.Background(), 2500)
	})

	return mbe, nil
}

var dbCalls []database.GetCallAudioRow
var mbe *mockAudioBackend
var backendMake sync.Once
