package callstore_test

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"dynatron.me/x/stillbox/pkg/calls/callstore"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/database"
	dbmock "dynatron.me/x/stillbox/pkg/database/mocks"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMove(t *testing.T) {
	storeCfg := config.CallStorage{
		Backends: []config.StorageBackendConfig{
			{
				Name:    "test",
				Backend: "test",
			},
		},
	}

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

	ctx := fillCtxRbac(t, t.Context())

	for _, tc := range tests {
		ctx, cancel := context.WithCancel(ctx)
		db := dbmock.NewStore(t)
		if tc.expectNumRows > 0 {
			db.EXPECT().InTx(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, f func(database.Store) error) error {
				return f(db)
			})
		}
		db.EXPECT().GetCallAudioCount(mock.Anything, mock.AnythingOfType("database.GetCallAudioParams")).RunAndReturn(func(ctx context.Context, gp database.GetCallAudioParams) (int64, error) {
			time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
			return tc.expectTotalRows, nil
		})
		refJournalMockExpect(db)
		gcaIter := int64(-1)
		if tc.expectNumRows > 0 {
			db.EXPECT().GetCallAudioCb(mock.Anything, mock.AnythingOfType("database.GetCallAudioParams"), mock.Anything).RunAndReturn(func(ctx context.Context, gp database.GetCallAudioParams, cb func(*database.GetCallAudioRow) error) error {
				time.Sleep(time.Duration(rand.Intn(4)) * time.Millisecond)
				for range dbCalls[0:tc.expectNumRows] {
					gcaIter++
					cb(&dbCalls[gcaIter])
				}
				return nil
			})
			db.EXPECT().SetCallAudio(mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ uuid.UUID, ref, blob []byte) error {
				time.Sleep(time.Duration(rand.Intn(2)) * time.Millisecond)
				return nil
			})
		}
		st := setupMockDBStore(ctx, t, db, storeCfg, tc.partConfig)
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

func BenchmarkMove(b *testing.B) {
	expectNumRows := int64(1000)
	expectTotalRows := int64(2000)
	storeCfg := config.CallStorage{
		Backends: []config.StorageBackendConfig{
			{
				Name:    "test",
				Backend: "test",
			},
		},
	}

	ctx := fillCtxRbacBench(b.Context())
	log.Logger = log.Level(zerolog.Disabled)

	for b.Loop() {
		ctx, _ = context.WithCancel(ctx)
		db := new(dbmock.Store)
		db.EXPECT().InTx(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, f func(database.Store) error) error {
			return f(db)
		})
		db.EXPECT().GetCallAudioCount(mock.Anything, mock.AnythingOfType("database.GetCallAudioParams")).RunAndReturn(func(ctx context.Context, gp database.GetCallAudioParams) (int64, error) {
			time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
			return expectTotalRows, nil
		})
		refJournalMockExpect(db)
		gcaIter := int64(-1)
		db.EXPECT().GetCallAudioCb(mock.Anything, mock.AnythingOfType("database.GetCallAudioParams"), mock.Anything).RunAndReturn(func(ctx context.Context, gp database.GetCallAudioParams, cb func(*database.GetCallAudioRow) error) error {
			time.Sleep(time.Duration(rand.Intn(4)) * time.Millisecond)
			for range dbCalls[0:expectNumRows] {
				gcaIter++
				cb(&dbCalls[gcaIter])
			}
			return nil
		})
		db.EXPECT().SetCallAudio(mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ uuid.UUID, ref, blob []byte) error {
			time.Sleep(time.Duration(rand.Intn(2)) * time.Millisecond)
			return nil
		})
		st, err := setupMockDBStoreB(ctx, db, storeCfg, config.Partition{})
		if err != nil {
			panic(err)
		}
		_, err = st.MoveCallAudio(ctx, callstore.MoveCallParams{})
		if err != nil {
			panic(err)
		}
	}
}
