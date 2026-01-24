package callstore_test

import (
	"context"
	"sync"
	"testing"

	"dynatron.me/x/stillbox/pkg/calls/callstore"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/database"
	dbmock "dynatron.me/x/stillbox/pkg/database/mocks"
	"dynatron.me/x/stillbox/pkg/metrics"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var mockRegisterBackend sync.Once

func refJournalMockExpect(db *dbmock.Store) {
	db.EXPECT().DetailedCountRefJournal(mock.AnythingOfType("*context.cancelCtx")).RunAndReturn(func(ctx context.Context) ([]database.DetailedCountRefJournalRow, error) {
		return nil, nil
	})
	db.EXPECT().GetAudioRefJournalCb(mock.AnythingOfType("*context.cancelCtx"), mock.Anything, mock.Anything).Return(nil)
}

func setupMockDBStore(ctx context.Context, t *testing.T, db database.Store, storeCfg config.CallStorage, partCfg config.Partition) callstore.Store {
	st, err := setupMockDBStoreB(ctx, db, storeCfg, partCfg)
	require.NoError(t, err)

	return st
}

func setupMockDBStoreB(ctx context.Context, db database.Store, storeCfg config.CallStorage, partCfg config.Partition) (callstore.Store, error) {
	mockRegisterBackend.Do(func() {
		callstore.RegisterAudioBackend("test", newMockAudioBackend)
	})

	met := metrics.NewNoOp()
	tgc := tgstore.NewCache(db, met)
	return callstore.NewStore(ctx, db, tgc, met, storeCfg, partCfg)
}
