package partman_test

import (
	"context"
	"testing"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/database/mocks"
	"dynatron.me/x/stillbox/pkg/database/partman"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var mctx = mock.Anything

func inTx(s *mocks.Store) {
	s.EXPECT().InTx(mctx, mock.AnythingOfType("func(database.Store) error"), mock.AnythingOfType("pgx.TxOptions")).RunAndReturn(func(ctx context.Context, f func(db database.Store) error, po pgx.TxOptions) error {
		return f(s)
	})

}

type timeRange struct {
	start time.Time
	end   time.Time
}

type partSpec struct {
	name string
	timeRange
}

func TestPartman(t *testing.T) {
	ctx := context.Background()

	timeInUTC := func(s string) time.Time {
		t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.UTC)
		if err != nil {
			panic(err)
		}

		return t
	}

	dateInUTC := func(s string) time.Time {
		t, err := time.ParseInLocation("2006-01-02", s, time.UTC)
		if err != nil {
			panic(err)
		}

		return t
	}

	partResultWithSchema := func(schema, name, low, up string) database.PartitionResult {
		return database.PartitionResult{
			ParentTable: "calls",
			Schema:      schema,
			Name:        name,
			LowerBound:  low,
			UpperBound:  up,
		}
	}

	partResult := func(name, low, up string) database.PartitionResult {
		return partResultWithSchema("public", name, low, up)
	}

	dailyTR := func(tr string) timeRange {
		dtr := dateInUTC(tr)
		etr := dtr.AddDate(0, 0, 1)
		return timeRange{start: dtr, end: etr}
	}

	tests := []struct {
		name          string
		now           time.Time
		cfg           config.Partition
		extant        []database.PartitionResult
		expectCreate  []partSpec
		expectDrop    []string
		expectDetach  []string
		expectSweep   []timeRange
		expectCleanup []timeRange
		expectErr     error
	}{
		{
			name: "monthly base",
			now:  timeInUTC("2024-11-28 11:37:04"),
			cfg: config.Partition{
				Enabled:      true,
				Schema:       "public",
				Interval:     "monthly",
				Retain:       2,
				Drop:         true,
				PreProvision: common.PtrTo(2),
			},
			extant: []database.PartitionResult{
				partResult("calls_p_2024_10", "2024-10-01", "2024-11-01"),
				partResult("calls_p_2024_09", "2024-09-01", "2024-10-01"),
				partResult("calls_p_2024_08", "2024-08-01", "2024-09-01"),
				partResult("calls_p_2024_07", "2024-07-01", "2024-08-01"),
			},
			expectCreate: []partSpec{
				{name: "calls_p_2024_11", timeRange: timeRange{start: dateInUTC("2024-11-01"), end: dateInUTC("2024-12-01")}},
				{name: "calls_p_2024_12", timeRange: timeRange{start: dateInUTC("2024-12-01"), end: dateInUTC("2025-01-01")}},
				{name: "calls_p_2025_01", timeRange: timeRange{start: dateInUTC("2025-01-01"), end: dateInUTC("2025-02-01")}},
			},
			expectDrop: []string{
				"public.calls_p_2024_07",
				"public.calls_p_2024_08",
			},
			expectSweep: []timeRange{
				{start: dateInUTC("2024-07-01"), end: dateInUTC("2024-08-01")},
				{start: dateInUTC("2024-08-01"), end: dateInUTC("2024-09-01")},
			},
			expectCleanup: []timeRange{
				{start: dateInUTC("2024-07-01"), end: dateInUTC("2024-08-01")},
				{start: dateInUTC("2024-08-01"), end: dateInUTC("2024-09-01")},
			},
			expectDetach: []string{
				"public.calls_p_2024_07",
				"public.calls_p_2024_08",
			},
		},
		{
			name: "monthly retain all",
			now:  timeInUTC("2024-11-28 11:37:04"),
			cfg: config.Partition{
				Enabled:      true,
				Schema:       "public",
				Interval:     "monthly",
				Retain:       -1,
				Drop:         true,
				PreProvision: common.PtrTo(2),
			},
			extant: []database.PartitionResult{
				partResult("calls_p_2024_10", "2024-10-01", "2024-11-01"),
				partResult("calls_p_2024_09", "2024-09-01", "2024-10-01"),
				partResult("calls_p_2024_08", "2024-08-01", "2024-09-01"),
				partResult("calls_p_2024_07", "2024-07-01", "2024-08-01"),
			},
			expectCreate: []partSpec{
				{name: "calls_p_2024_11", timeRange: timeRange{start: dateInUTC("2024-11-01"), end: dateInUTC("2024-12-01")}},
				{name: "calls_p_2024_12", timeRange: timeRange{start: dateInUTC("2024-12-01"), end: dateInUTC("2025-01-01")}},
				{name: "calls_p_2025_01", timeRange: timeRange{start: dateInUTC("2025-01-01"), end: dateInUTC("2025-02-01")}},
			},
			expectDrop:    []string{},
			expectSweep:   []timeRange{},
			expectCleanup: []timeRange{},
			expectDetach:  []string{},
		},

		{
			name: "weekly base",
			now:  timeInUTC("2024-11-28 11:37:04"), // week 48
			cfg: config.Partition{
				Enabled:      true,
				Schema:       "public",
				Interval:     "weekly",
				Retain:       2,
				Drop:         false,
				PreProvision: common.PtrTo(2),
			},
			extant: []database.PartitionResult{
				partResult("calls_p_2024_w44", "2024-10-28", "2024-11-04"),
				partResult("calls_p_2024_w45", "2024-11-04", "2024-11-11"),
				partResult("calls_p_2024_w46", "2024-11-11", "2024-11-18"),
				// missing week 47
			},
			expectCreate: []partSpec{
				{name: "calls_p_2024_w47", timeRange: timeRange{start: dateInUTC("2024-11-18"), end: dateInUTC("2024-11-25")}},
				{name: "calls_p_2024_w48", timeRange: timeRange{start: dateInUTC("2024-11-25"), end: dateInUTC("2024-12-02")}},
				{name: "calls_p_2024_w49", timeRange: timeRange{start: dateInUTC("2024-12-02"), end: dateInUTC("2024-12-09")}},
				{name: "calls_p_2024_w50", timeRange: timeRange{start: dateInUTC("2024-12-09"), end: dateInUTC("2024-12-16")}},
			},
			expectSweep: []timeRange{
				{start: dateInUTC("2024-10-28"), end: dateInUTC("2024-11-04")},
				{start: dateInUTC("2024-11-04"), end: dateInUTC("2024-11-11")},
			},
			expectCleanup: []timeRange{
				{start: dateInUTC("2024-10-28"), end: dateInUTC("2024-11-04")},
				{start: dateInUTC("2024-11-04"), end: dateInUTC("2024-11-11")},
			},
			expectDetach: []string{
				"public.calls_p_2024_w44",
				"public.calls_p_2024_w45",
			},
		},
		{
			name: "daily base",
			now:  timeInUTC("2024-12-31 11:37:04"),
			cfg: config.Partition{
				Enabled:      true,
				Schema:       "public",
				Interval:     "daily",
				Retain:       2,
				Drop:         true,
				PreProvision: common.PtrTo(2),
			},
			extant: []database.PartitionResult{
				partResult("calls_p_2024_12_26", "2024-12-26", "2024-12-27"),
				partResult("calls_p_2024_12_27", "2024-12-27", "2024-12-28"),
				partResult("calls_p_2024_12_30", "2024-12-30", "2024-12-31"),
				partResult("calls_p_2024_12_31", "2024-12-31", "2025-01-01"),
			},
			expectCreate: []partSpec{
				{name: "calls_p_2024_12_29", timeRange: dailyTR("2024-12-29")},
				{name: "calls_p_2025_01_01", timeRange: dailyTR("2025-01-01")},
				{name: "calls_p_2025_01_02", timeRange: dailyTR("2025-01-02")},
			},
			expectDrop: []string{
				"public.calls_p_2024_12_26",
				"public.calls_p_2024_12_27",
			},
			expectSweep: []timeRange{
				{start: dateInUTC("2024-12-26"), end: dateInUTC("2024-12-27")},
				{start: dateInUTC("2024-12-27"), end: dateInUTC("2024-12-28")},
			},
			expectCleanup: []timeRange{
				{start: dateInUTC("2024-12-26"), end: dateInUTC("2024-12-27")},
				{start: dateInUTC("2024-12-27"), end: dateInUTC("2024-12-28")},
			},
			expectDetach: []string{
				"public.calls_p_2024_12_26",
				"public.calls_p_2024_12_27",
			},
		},
		{
			name: "quarterly base",
			now:  timeInUTC("2025-07-28 11:37:04"), // q3
			cfg: config.Partition{
				Enabled:      true,
				Schema:       "public",
				Interval:     "quarterly",
				Retain:       2,
				Drop:         true,
				PreProvision: common.PtrTo(2),
			},
			extant: []database.PartitionResult{
				partResult("calls_p_2024_q3", "2024-07-01", "2024-10-01"),
				partResult("calls_p_2024_q4", "2024-10-01", "2025-01-01"),
				partResult("calls_p_2025_q1", "2025-01-01", "2024-04-01"),
				partResult("calls_p_2025_q2", "2025-04-01", "2024-07-01"),
			},
			expectDrop: []string{
				"public.calls_p_2024_q3",
				"public.calls_p_2024_q4",
			},
			expectSweep: []timeRange{
				{start: dateInUTC("2024-07-01"), end: dateInUTC("2024-10-01")},
				{start: dateInUTC("2024-10-01"), end: dateInUTC("2025-01-01")},
			},
			expectCleanup: []timeRange{
				{start: dateInUTC("2024-07-01"), end: dateInUTC("2024-10-01")},
				{start: dateInUTC("2024-10-01"), end: dateInUTC("2025-01-01")},
			},
			expectCreate: []partSpec{
				{name: "calls_p_2025_q3", timeRange: timeRange{dateInUTC("2025-07-01"), dateInUTC("2025-10-01")}},
				{name: "calls_p_2025_q4", timeRange: timeRange{dateInUTC("2025-10-01"), dateInUTC("2026-01-01")}},
				{name: "calls_p_2026_q1", timeRange: timeRange{dateInUTC("2026-01-01"), dateInUTC("2026-04-01")}},
			},
			expectDetach: []string{
				"public.calls_p_2024_q3",
				"public.calls_p_2024_q4",
			},
		},
		{
			name: "yearly base",
			now:  timeInUTC("2023-04-28 11:37:04"), // q3
			cfg: config.Partition{
				Enabled:      true,
				Schema:       "public",
				Interval:     "yearly",
				Retain:       3,
				Drop:         false,
				PreProvision: common.PtrTo(2),
			},
			extant: []database.PartitionResult{
				partResult("calls_p_2019", "2019-01-01", "2020-01-01"),
				partResult("calls_p_2020", "2020-01-01", "2021-01-01"),
				partResult("calls_p_2021", "2021-01-01", "2022-01-01"),
				partResult("calls_p_2022", "2022-01-01", "2023-01-01"),
				partResult("calls_p_2023", "2023-01-01", "2024-01-01"),
			},
			expectDrop: []string{},
			expectSweep: []timeRange{
				{start: dateInUTC("2019-01-01"), end: dateInUTC("2020-01-01")},
			},
			expectCleanup: []timeRange{
				{start: dateInUTC("2019-01-01"), end: dateInUTC("2020-01-01")},
			},
			expectCreate: []partSpec{
				{name: "calls_p_2024", timeRange: timeRange{dateInUTC("2024-01-01"), dateInUTC("2025-01-01")}},
				{name: "calls_p_2025", timeRange: timeRange{dateInUTC("2025-01-01"), dateInUTC("2026-01-01")}},
			},
			expectDetach: []string{
				"public.calls_p_2019",
			},
		},
		{
			name: "changed monthly to daily",
			now:  timeInUTC("2024-11-28 11:37:04"),
			cfg: config.Partition{
				Enabled:      true,
				Schema:       "public",
				Interval:     "daily",
				Retain:       2,
				Drop:         true,
				PreProvision: common.PtrTo(2),
			},
			extant: []database.PartitionResult{
				partResult("calls_p_2024_10", "2024-10-01", "2024-11-01"),
				partResult("calls_p_2024_09", "2024-09-01", "2024-10-01"),
				partResult("calls_p_2024_08", "2024-08-01", "2024-09-01"),
				partResult("calls_p_2024_07", "2024-07-01", "2024-08-01"),
			},
			expectErr: partman.ErrDifferentInterval,
		},
		{
			name: "monthly wrong schema",
			now:  timeInUTC("2024-11-28 11:37:04"),
			cfg: config.Partition{
				Enabled:      true,
				Schema:       "public",
				Interval:     "monthly",
				Retain:       2,
				Drop:         true,
				PreProvision: common.PtrTo(2),
			},
			extant: []database.PartitionResult{
				partResult("calls_p_2024_10", "2024-10-01", "2024-11-01"),
				partResultWithSchema("reid", "calls_p_2024_09", "2024-09-01", "2024-10-01"),
				partResult("calls_p_2024_08", "2024-08-01", "2024-09-01"),
				partResult("calls_p_2024_07", "2024-07-01", "2024-08-01"),
			},
			expectErr: partman.ErrWrongSchema,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := mocks.NewStore(t)
			createdPartitions := make([]partSpec, 0, len(tc.expectCreate))
			sweptRanges := make([]timeRange, 0, len(tc.expectSweep))
			droppedPartitions := make([]string, 0, len(tc.expectDrop))
			cleanupRanges := make([]timeRange, 0, len(tc.expectCleanup))
			detachedPartitions := make([]string, 0, len(tc.expectDetach))
			sweepMap := make(map[timeRange]struct{})

			if len(tc.expectCreate) > 0 {
				db.EXPECT().
					CreatePartition(
						mctx, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("time.Time"),
						mock.AnythingOfType("time.Time"),
					).
					Run(func(ctx context.Context, tableName, partitionName string, start, end time.Time) {
						ps := partSpec{name: partitionName, timeRange: timeRange{start: start, end: end}}
						createdPartitions = append(createdPartitions, ps)
					}).Return(nil)
			}

			if len(tc.expectSweep) > 0 {
				db.EXPECT().
					SweepCalls(
						mctx, mock.AnythingOfType("pgtype.Timestamptz"), mock.AnythingOfType("pgtype.Timestamptz"),
					).
					Run(func(ctx context.Context, start, end pgtype.Timestamptz) {
						tr := timeRange{start: start.Time, end: end.Time}
						sweepMap[tr] = struct{}{}
						sweptRanges = append(sweptRanges, tr)
					}).Return(30, nil)
			}

			if len(tc.expectCleanup) > 0 {
				db.EXPECT().
					CleanupSweptCalls(
						mctx, mock.AnythingOfType("pgtype.Timestamptz"), mock.AnythingOfType("pgtype.Timestamptz"),
					).Run(func(ctx context.Context, start, end pgtype.Timestamptz) {
					tr := timeRange{start: start.Time, end: end.Time}
					require.Contains(t, sweepMap, tr)

					cleanupRanges = append(cleanupRanges, tr)
				}).Return(30, nil)
			}

			if tc.cfg.Drop && len(tc.expectDrop) > 0 {
				db.EXPECT().
					DropPartition(mctx, mock.AnythingOfType("string")).
					Run(func(ctx context.Context, partName string) {
						droppedPartitions = append(droppedPartitions, partName)
					}).Return(nil)
			}

			if len(tc.expectDetach) > 0 {
				db.EXPECT().
					DetachPartition(
						mctx, mock.AnythingOfType("string"), mock.AnythingOfType("string")).
					Run(func(ctx context.Context, parentTable, partName string) {
						detachedPartitions = append(detachedPartitions, partName)
					}).Return(nil)
			}

			inTx(db)

			db.EXPECT().GetTablePartitions(mctx, "public", "calls").Return(tc.extant, nil)

			pm, err := partman.New(db, tc.cfg)
			require.NoError(t, err)

			err = pm.Check(ctx, tc.now)
			if tc.expectErr != nil {
				assert.ErrorIs(t, err, tc.expectErr)
			} else {
				require.NoError(t, err)

				assert.ElementsMatch(t, tc.expectCreate, createdPartitions, "created partitions")
				assert.ElementsMatch(t, tc.expectSweep, sweptRanges, "swept ranges")
				assert.ElementsMatch(t, tc.expectDrop, droppedPartitions, "dropped partitions")
				assert.ElementsMatch(t, tc.expectCleanup, cleanupRanges, "cleaned up ranges")
				assert.ElementsMatch(t, tc.expectDetach, detachedPartitions, "detached partitions")
			}
		})
	}
}
