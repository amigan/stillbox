package callstore

// portions lifted gratefully from github.com/qonto/postgresql-partition-manager, MIT license.

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/isoweek"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/database"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
)

const (
	CallsTable = "calls"

	CheckInterval = time.Hour // every 1h

	preProvisionDefault = 1
)

var (
	ErrWrongSchema       = errors.New("wrong schema name")
	ErrDifferentInterval = errors.New("stored partition interval differs from configured")
)

type PartitionErr struct {
	p   string
	err error
}

func (pe PartitionErr) Error() string {
	r := fmt.Sprintf("bad partition '%s'", pe.p)
	if pe.err != nil {
		r += ": " + pe.err.Error()
	}

	return r
}

func (pe PartitionErr) Unwrap() error {
	return pe.err
}

type ParsedIntvlErr struct {
	parsed, start time.Time
}

func (pie ParsedIntvlErr) Error() string {
	return fmt.Sprintf("parsed interval (%s) does not match start (%s)", pie.parsed, pie.start)
}

func PartitionError(pname string, err ...error) PartitionErr {
	if len(err) > 0 {
		return PartitionErr{p: pname, err: err[0]}
	}

	return PartitionErr{p: pname}
}

type PartitionManager interface {
	Go(ctx context.Context)
	Check(ctx context.Context, now time.Time) error
	Interval() common.Interval
	ExistingPartitions(parts []database.PartitionResult) ([]Partition, error)
	PartitionPrefix(time.Time) string
}

type partman struct {
	db      database.Store
	calls   PartmanCallAudioManager
	cfg     config.Partition
	intv    common.Interval
	bounder *common.TimeBounder
}

type PartmanCallAudioManager interface {
	// DerefSweptCallAudios resolves all audio_refs of swept calls and puts the audio into audio_blob.
	DerefSweptCallAudios(ctx context.Context, tx database.Store) error

	// PruneAudioPrefix schedules for pruning (backend-specific) all call audios with partPrefix.
	PruneAudioPrefix(ctx context.Context, tx database.Store, partPrefix string, pStart, pEnd pgtype.Timestamptz) error
}

func (pm *partman) Interval() common.Interval {
	return pm.intv
}

type Partition struct {
	ParentTable string
	Schema      string
	Name        string
	Interval    common.Interval
	Time        time.Time
}

func NewPartitionManager(db database.Store, callStore *store, cfg config.Partition) (*partman, error) {
	intv := common.Interval(cfg.Interval)
	if !intv.IsValid() {
		return nil, common.ErrInvalidInterval(intv)
	}

	pm := &partman{
		cfg:     cfg,
		db:      db,
		calls:   callStore,
		intv:    intv,
		bounder: common.NewTimeBounder(common.WithDefaultBounds(intv)),
	}

	return pm, nil
}

var _ PartitionManager = (*partman)(nil)

func (pm *partman) Go(ctx context.Context) {
	ctx = entities.CtxWithServiceSubject(ctx, "partman")
	tick := time.NewTicker(CheckInterval)

	select {
	case now := <-tick.C:
		err := pm.Check(ctx, now)
		if err != nil {
			log.Error().Err(err).Msg("partman check failed")
		}
	case <-ctx.Done():
		return
	}
}

func (pm *partman) newPartition(t time.Time) Partition {
	p := Partition{
		ParentTable: CallsTable,
		Schema:      pm.cfg.Schema,
		Interval:    common.Interval(pm.cfg.Interval),
		Time:        t,
	}

	p.setName()

	return p
}

func (pm *partman) retentionPartitions(cur Partition) []Partition {
	partitions := make([]Partition, 0, pm.cfg.Retain)
	for i := 1; i <= pm.cfg.Retain; i++ {
		prev := cur.Prev(i)
		partitions = append(partitions, prev)
	}

	return partitions
}

func (pm *partman) futurePartitions(cur Partition) []Partition {
	preProv := preProvisionDefault
	if pm.cfg.PreProvision != nil {
		preProv = *pm.cfg.PreProvision
	}

	partitions := make([]Partition, 0, preProv)
	for i := 1; i <= preProv; i++ {
		next := cur.Next(i)
		partitions = append(partitions, next)
	}

	return partitions
}

func (pm *partman) expectedPartitions(now time.Time) []Partition {
	curPart := pm.newPartition(now)

	shouldExist := []Partition{curPart}
	if pm.cfg.Retain > -1 {
		retain := pm.retentionPartitions(curPart)
		shouldExist = append(shouldExist, retain...)
	}

	future := pm.futurePartitions(curPart)

	shouldExist = append(shouldExist, future...)

	return shouldExist
}

func (pm *partman) comparePartitions(existingTables, expectedTables []Partition) (unexpectedTables, missingTables []Partition) {
	existing := make(map[string]Partition)
	expectedAndExists := make(map[string]bool)

	for _, t := range existingTables {
		existing[t.PartitionName()] = t
	}

	for _, t := range expectedTables {
		if _, found := existing[t.PartitionName()]; found {
			expectedAndExists[t.PartitionName()] = true
		} else {
			missingTables = append(missingTables, t)
		}
	}

	for _, t := range existingTables {
		if _, found := expectedAndExists[t.PartitionName()]; !found {
			// Only in existingTables and not in both
			unexpectedTables = append(unexpectedTables, t)
		}
	}

	return unexpectedTables, missingTables
}

func (pm *partman) ExistingPartitions(parts []database.PartitionResult) ([]Partition, error) {
	existing := make([]Partition, 0, len(parts))
	for _, v := range parts {
		if v.Schema != pm.cfg.Schema {
			return nil, PartitionError(v.Schema+"."+v.Name, ErrWrongSchema)
		}
		p, err := pm.verifyPartName(v)
		if err != nil {
			return nil, err
		}

		if p.Interval != common.Interval(pm.cfg.Interval) {
			return nil, PartitionError(v.Schema+"."+v.Name, ErrDifferentInterval)
		}

		existing = append(existing, p)
	}
	return existing, nil
}

func (pm *partman) fullTableName(s string) string {
	return fmt.Sprintf("%s.%s", pm.cfg.Schema, s)
}

func (pm *partman) prunePartition(ctx context.Context, tx database.Store, p Partition) error {
	s, e := p.Range()
	start := pgtype.Timestamptz{Time: s, Valid: true}
	end := pgtype.Timestamptz{Time: e, Valid: true}
	fullPartName := pm.fullTableName(p.PartitionName())

	// sweep calls that are referenced by an incident into swept_calls
	swept, err := tx.SweepCalls(ctx, start, end)
	if err != nil {
		return err
	}
	log.Info().Int64("rows", swept).Time("start", s).Time("end", e).Msg("swept calls")

	swept, err = tx.CleanupSweptCalls(ctx, start, end)
	if err != nil {
		return err
	}
	log.Debug().Int64("rows", swept).Time("start", s).Time("end", e).Msg("cleaned up swept calls")

	// go through swept calls and resolve all audio references.
	// in the future, we may make this configurable along with the necessary changes to
	// audioBackends to exclude calls from the pruning that happens below.
	err = pm.calls.DerefSweptCallAudios(ctx, tx)
	if err != nil {
		return err
	}

	err = pm.calls.PruneAudioPrefix(ctx, tx, p.Prefix(), start, end)
	if err != nil {
		return err
	}

	log.Info().Str("partition", fullPartName).Msg("detaching partition")
	err = tx.DetachPartition(ctx, CallsTable, fullPartName)
	if err != nil {
		return err
	}

	if pm.cfg.Drop {
		log.Info().Str("partition", fullPartName).Msg("dropping partition")
		return tx.DropPartition(ctx, fullPartName)
	}

	return nil
}

func (pm *partman) Check(ctx context.Context, now time.Time) error {
	return pm.db.InTx(ctx, func(db database.Store) error {
		// by default, we want to make sure a partition exists for this and next interval
		// since we run this at startup, it's safe to do only that.
		partitions, err := db.GetTablePartitions(ctx, pm.cfg.Schema, CallsTable)
		if err != nil {
			return err
		}

		existing, err := pm.ExistingPartitions(partitions)
		if err != nil {
			return err
		}

		expected := pm.expectedPartitions(now)

		unexpected, missing := pm.comparePartitions(existing, expected)

		if pm.cfg.Retain > -1 {
			for _, p := range unexpected {
				err := pm.prunePartition(ctx, db, p)
				if err != nil {
					return err
				}
			}
		}

		for _, p := range missing {
			err := pm.createPartition(ctx, db, p)
			if err != nil {
				return err
			}
		}

		return nil

	}, pgx.TxOptions{})
}

func (p Partition) PartitionName() string {
	return p.Name
}

func (p Partition) Prefix() string {
	return strings.Split(p.Name, "calls_p_")[1]
}

func (pm *partman) createPartition(ctx context.Context, tx database.Store, part Partition) error {
	start, end := part.Range()
	name := part.PartitionName()
	log.Info().Str("partition", name).Time("start", start).Time("end", end).Msg("creating partition")
	return tx.CreatePartition(ctx, CallsTable, name, start, end)
}

/*
 * Partition scheme names:
 * daily: calls_p_2024_11_28
 * weekly: calls_p_2024_w48
 * monthly: calls_p_2024_11
 * quarterly: calls_p_2024_q4
 * yearly: calls_p_2024
 */

func (pm *partman) verifyPartName(pr database.PartitionResult) (p Partition, err error) {
	pn := pr.Name
	low, _, err := pr.ParseBounds()
	if err != nil {
		return
	}
	p = Partition{
		ParentTable: pr.ParentTable,
		Name:        pr.Name,
		Schema:      pr.Schema,
		Time:        low,
	}
	dateAr := strings.Split(pn, "calls_p_")
	if len(dateAr) != 2 {
		return p, PartitionError(pn)
	}

	dateAr = strings.Split(dateAr[1], "_")
	switch len(dateAr) {
	case 3: // daily
		p.Interval = common.Daily
		ymd := [3]int{}
		for i := range 3 {
			r, err := strconv.Atoi(dateAr[i])
			if err != nil {
				return p, PartitionError(pn, err)
			}

			ymd[i] = r
		}
		parsed := time.Date(ymd[0], time.Month(ymd[1]), ymd[2], 0, 0, 0, 0, time.UTC)
		if !parsed.Equal(p.Time) {
			return p, PartitionError(pn, ParsedIntvlErr{parsed: parsed, start: p.Time})
		}
		return p, nil
	case 2:
		year, err := strconv.Atoi(dateAr[0])
		if err != nil {
			return p, PartitionError(pn, err)
		}
		if strings.HasPrefix(dateAr[1], "w") {
			p.Interval = common.Weekly
			weekNum, err := strconv.Atoi(dateAr[1][1:])
			if err != nil {
				return p, PartitionError(pn, err)
			}

			parsed := isoweek.StartTime(year, weekNum, time.UTC)
			if parsed != p.Time {
				return p, PartitionError(pn, ParsedIntvlErr{parsed: parsed, start: p.Time})
			}
			return p, nil
		} else if strings.HasPrefix(dateAr[1], "q") {
			p.Interval = common.Quarterly
			quarterNum, err := strconv.Atoi(dateAr[1][1:])
			if err != nil {
				return p, PartitionError(pn, err)
			}
			if quarterNum > 4 {
				return p, PartitionError(pn, errors.New("invalid quarter"))
			}
			firstMonthOfTheQuarter := time.Month((quarterNum-1)*common.MonthsInQuarter + 1)
			parsed := time.Date(year, firstMonthOfTheQuarter, 1, 0, 0, 0, 0, time.UTC)
			if parsed != p.Time {
				return p, PartitionError(pn, ParsedIntvlErr{parsed: parsed, start: p.Time})
			}
			return p, nil
		}
		// monthly
		p.Interval = common.Monthly
		month, err := strconv.Atoi(dateAr[1])
		if err != nil {
			return p, PartitionError(pn)
		}

		parsed := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
		if parsed != p.Time {
			return p, PartitionError(pn, ParsedIntvlErr{parsed: parsed, start: p.Time})
		}
		return p, nil
	case 1: // yearly
		p.Interval = common.Yearly
		year, err := strconv.Atoi(dateAr[0])
		if err != nil {
			return p, PartitionError(pn, err)
		}
		parsed := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		if parsed != p.Time {
			return p, PartitionError(pn, ParsedIntvlErr{parsed: parsed, start: p.Time})
		}

		return p, nil
	}

	return p, PartitionError(pn)
}

func (pm *partman) PartitionPrefix(t time.Time) string {
	lb, _ := pm.bounder.Bounds(t)

	switch pm.intv {
	case common.Daily:
		return lb.Format("2006_01_02")
	case common.Weekly:
		year, week := isoweek.FromDate(t.Year(), t.Month(), t.Day())
		return fmt.Sprintf("%04d_w%02d", year, week)
	case common.Monthly:
		return lb.Format("2006_01")
	case common.Quarterly:
		quarter := ((int(lb.Month()) - 1) / common.MonthsInQuarter) + 1
		return fmt.Sprintf("%04d_q%d", t.Year(), quarter)
	case common.Yearly:
		return lb.Format("2006")
	}

	return ""
}
