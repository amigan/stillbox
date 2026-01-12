//go:build integration

package callstore_test

import (
	"context"
	"sync"
	"testing"

	"dynatron.me/x/stillbox/internal/testutil"
	"dynatron.me/x/stillbox/pkg/calls/callstore"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/database"
	dbmock "dynatron.me/x/stillbox/pkg/database/mocks"
	"dynatron.me/x/stillbox/pkg/metrics"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

var mockRegisterBackend sync.Once

func refJournalMockExpect(db *dbmock.Store) {
	db.EXPECT().DetailedCountRefJournal(mock.AnythingOfType("*context.cancelCtx")).RunAndReturn(func(ctx context.Context) ([]database.DetailedCountRefJournalRow, error) {
		return nil, nil
	})
	db.EXPECT().GetAudioRefJournalCb(mock.AnythingOfType("*context.cancelCtx"), mock.Anything, mock.Anything).Return(nil)
}

func setupMockDBStore(ctx context.Context, t *testing.T, db database.Store, storeCfg config.CallStorage, partCfg config.Partition) callstore.Store {
	mockRegisterBackend.Do(func() {
		callstore.RegisterAudioBackend("test", newMockAudioBackend)
	})

	met := metrics.NewNoOp()
	tgc := tgstore.NewCache(db, met)
	st, err := callstore.NewStore(ctx, db, tgc, met, storeCfg, partCfg)
	require.NoError(t, err)

	return st
}

type DBTestSuite struct {
	suite.Suite
	db    *testutil.DB
	store callstore.Store
	tgs   tgstore.Store
}

func (s *DBTestSuite) TearDownTest() {
	s.db.Cleanup()
}

func NewDBTestSuite(ctx context.Context, t *testing.T, storeCfg config.CallStorage, partCfg testutil.PartConfig, stOpt ...testutil.DBOpt) (*DBTestSuite, context.Context) {
	met := metrics.NewNoOp()
	db := testutil.NewDB(partCfg, stOpt...)
	t.Logf("schema %s", db.SchemaName)
	tgc := tgstore.NewCache(db, met)
	st, err := callstore.NewStore(ctx, db, tgc, met, storeCfg, db.PartConfig, callstore.WithNow(callstore.NowFunc(db.NowFunc())))
	require.NoError(t, err)

	dbts := &DBTestSuite{
		db:    db,
		store: st,
	}

	ctx = database.CtxWithDB(ctx, db)
	ctx = tgstore.CtxWithStore(ctx, tgc)
	ctx = callstore.CtxWithStore(ctx, st)

	return dbts, ctx
}
