//go:build integration

package callstore_test

import (
	"context"
	"testing"

	"dynatron.me/x/stillbox/internal/testutil"
	"dynatron.me/x/stillbox/pkg/calls/callstore"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/metrics"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

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
