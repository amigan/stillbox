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

var registerBackendOnce sync.Once

func refJournalMockExpect(db *dbmock.Store) {
	db.EXPECT().DetailedCountRefJournal(mock.AnythingOfType("*context.cancelCtx")).RunAndReturn(func(ctx context.Context) ([]database.DetailedCountRefJournalRow, error) {
		return nil, nil
	})
	db.EXPECT().GetAudioRefJournalCb(mock.AnythingOfType("*context.cancelCtx"), mock.Anything, mock.Anything).Return(nil)
}

func setupStore(ctx context.Context, t *testing.T, db database.Store, partCfg config.Partition) callstore.Store {
	registerBackendOnce.Do(func() {
		callstore.RegisterAudioBackend("test", newMockAudioBackend)
	})

	met := metrics.NewNoOp()
	tgc := tgstore.NewCache(db, met)
	st, err := callstore.NewStore(ctx, db, tgc, met, callStorage, partCfg)
	require.NoError(t, err)

	return st
}
