package callstore_test

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"sync"
	"testing"
	"time"

	"dynatron.me/x/stillbox/pkg/authz"
	rbacmock "dynatron.me/x/stillbox/pkg/authz/mocks"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/calls/callstore"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/database"
	dbmock "dynatron.me/x/stillbox/pkg/database/mocks"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type testSub struct{}               // TODO: move to test package
func (*testSub) String() string     { return "test" }
func (*testSub) GetName() string    { return "test" }
func (*testSub) GetRoles() []string { return []string{"admin"} }

func fillCtx(t *testing.T, ctx context.Context) context.Context {
	rm := rbacmock.NewRBAC(t)

	rm.EXPECT().Check(mock.AnythingOfType("*context.cancelCtx"), mock.Anything, mock.Anything).Return(new(testSub), nil)
	return authz.CtxWithRBAC(ctx, rm)
}

var callStorage config.CallStorage = config.CallStorage{
	Backends: []config.StorageBackendConfig{
		{
			Name:    "test",
			Backend: "test",
		},
	},
}

type mockAudioBackend struct {
	calls map[string]*calls.CallAudio
	st    callstore.Store
}

func (m *mockAudioBackend) Store(ctx context.Context, ca *calls.CallAudio) (callstore.AudioRef, error) {
	m.calls[ca.ID.String()] = ca
	return ca.ID.String(), nil
}

func (m *mockAudioBackend) Get(ctx context.Context, call *calls.CallAudio, audioRef callstore.AudioRef, opts *callstore.CallAudioOptions) (blob []byte, audioURL *url.URL, err error) {
	*call = *m.calls[audioRef.(string)]
	return call.AudioBlob, call.AudioURL, nil
}

func (m *mockAudioBackend) Delete(ctx context.Context, audioRef callstore.AudioRef) error {
	delete(m.calls, audioRef.(string))
	return nil
}

func (m *mockAudioBackend) DeleteBulk(ctx context.Context, refs []callstore.AudioRef) error {
	return nil
}

func (m *mockAudioBackend) Type() string {
	return "test"
}

func (m *mockAudioBackend) Prune(ctx context.Context, audioRef callstore.AudioRef, pruneAfter *time.Time) (newPruneAfter *time.Time, err error) {
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

func TestMove(t *testing.T) {
	tests := []struct {
		desc            string
		par             callstore.MoveCallParams
		expectErr       error
		expectNumRows   int64
		expectTotalRows int64
		partConfig      config.Partition
		canceler        func(cancel func())
	}{
		{
			desc:            "base",
			par:             callstore.MoveCallParams{},
			expectNumRows:   1000,
			expectTotalRows: 2000,
		},
		{
			desc:            "base canceler",
			par:             callstore.MoveCallParams{},
			expectNumRows:   1000,
			expectTotalRows: 2000,
			canceler: func(cancel func()) {
				time.Sleep(time.Millisecond * 200)
				cancel()
			},
		},
		{
			desc:          "base zero rows",
			par:           callstore.MoveCallParams{},
			expectNumRows: 0,
		},
	}

	ctx := fillCtx(t, t.Context())

	for _, tc := range tests {
		ctx, cancel := context.WithCancel(ctx)
		db := dbmock.NewStore(t)
		db.EXPECT().InTx(mock.Anything, mock.Anything, pgx.TxOptions{}).RunAndReturn(func(ctx context.Context, f func(database.Store) error, _ pgx.TxOptions) error {
			return f(db)
		})
		db.EXPECT().GetCallAudioCount(mock.Anything, mock.AnythingOfType("database.GetCallAudioParams")).RunAndReturn(func(ctx context.Context, gp database.GetCallAudioParams) (int64, error) {
			time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
			return tc.expectTotalRows, nil
		})
		refJournalMockExpect(db)
		gcaIter := int64(-1)
		if tc.expectNumRows > 0 {
			db.EXPECT().GetCallAudio(mock.Anything, mock.AnythingOfType("database.GetCallAudioParams")).RunAndReturn(func(ctx context.Context, gp database.GetCallAudioParams) ([]database.GetCallAudioRow, error) {
				time.Sleep(time.Duration(rand.Intn(4)) * time.Millisecond)
				gcaIter++
				return dbCalls[gcaIter : gcaIter+tc.expectNumRows], nil
			})
			db.EXPECT().SetCallAudio(mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ uuid.UUID, ref, blob []byte) error {
				time.Sleep(time.Duration(rand.Intn(2)) * time.Millisecond)
				return nil
			})
		}
		st := setupStore(ctx, t, db, tc.partConfig)
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

			if tc.canceler == nil {
				assert.Equal(t, tc.expectTotalRows, nr)
			}
		})
	}
}
