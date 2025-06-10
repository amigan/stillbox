package callstore

import (
	"context"
	"testing"

	"dynatron.me/x/stillbox/pkg/config"
	dbmock "dynatron.me/x/stillbox/pkg/database/mocks"
	"dynatron.me/x/stillbox/pkg/metrics"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
	"github.com/stretchr/testify/require"
)

var beCfg []config.CallStorage = []config.CallStorage{
	{
	},
}

func TestMove(t *testing.T) {
	tests := []struct{
	}{
	}

	ctx := context.Background()

	for _, tc := range tests {
		db := dbmock.NewStore(t)
		// XXX: db calls used here should sleep randomly
		met := metrics.NewNoOp()
		tgc := tgstore.NewCache(db, met)
		st, err := NewStore(ctx, db, tgc, met, nil)
		require.NoError(t, err)
	}
}
