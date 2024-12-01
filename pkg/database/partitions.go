package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrLowerBoundAfterUpperBound = errors.New("lower bound after upper bound")
	ErrCantDecodePartitionBounds = errors.New("cannot decode partition bounds")
)

type PartitionResult struct {
	ParentTable string
	Schema      string
	Name        string
	LowerBound  string
	UpperBound  string
}

type partitionsQuerier interface {
	GetTablePartitions(ctx context.Context, schemaName, tableName string) ([]PartitionResult, error)
	CreatePartition(ctx context.Context, parentTable, partitionName string, start, end time.Time) error
	DetachPartition(ctx context.Context, parentTable, partitionName string) error
	DropPartition(ctx context.Context, partitionName string) error
}

func (q *Queries) GetTablePartitions(ctx context.Context, schemaName, tableName string) (partitions []PartitionResult, err error) {
	query := fmt.Sprintf(`
	WITH parts as (
		SELECT
		   relnamespace::regnamespace as schema,
		   c.oid::pg_catalog.regclass AS part_name,
		   regexp_match(pg_get_expr(c.relpartbound, c.oid),
			  'FOR VALUES FROM \(''(.*)''\) TO \(''(.*)''\)') AS bounds
		 FROM
		   pg_catalog.pg_class c JOIN pg_catalog.pg_inherits i ON (c.oid = i.inhrelid)
		 WHERE i.inhparent = '%s.%s'::regclass
		   AND c.relkind='r'
	)
	SELECT
		schema,
		part_name as name,
		'%s' as parentTable,
		bounds[1]::text AS lowerBound,
		bounds[2]::text AS upperBound
	FROM parts
	ORDER BY part_name;`, schemaName, tableName, tableName)

	rows, err := q.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get partitions: %w", err)
	}

	partitions, err = pgx.CollectRows(rows, pgx.RowToStructByName[PartitionResult])
	if err != nil {
		return nil, fmt.Errorf("failed to cast list: %w", err)
	}

	return partitions, nil
}

func (q *Queries) CreatePartition(ctx context.Context, parentTable, partitionName string, start, end time.Time) error {
	const boundFmt = "2006-01-02 00:00:00Z00"
	_, err := q.db.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s PARTITION OF %s FOR VALUES FROM ('%s') TO ('%s');`, partitionName, parentTable, start.Format(boundFmt), end.Format(boundFmt)))

	return err
}

func (q *Queries) DropPartition(ctx context.Context, partitionName string) error {
	_, err := q.db.Exec(ctx, fmt.Sprintf(`DROP TABLE %s;`, partitionName))

	return err
}

func (q *Queries) DetachPartition(ctx context.Context, parentTable, partitionName string) error {
	_, err := q.db.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s DETACH PARTITION %s;`, parentTable, partitionName))
	return err
}

func (partition PartitionResult) ParseBounds() (lowerBound time.Time, upperBound time.Time, err error) {
	lowerBound, upperBound, err = parseBoundAsDate(partition)
	if err == nil {
		return lowerBound, upperBound, nil
	}

	lowerBound, upperBound, err = parseBoundAsDateTime(partition)
	if err == nil {
		return lowerBound, upperBound, nil
	}

	lowerBound, upperBound, err = parseBoundAsDateTimeWithTimezone(partition)
	if err == nil {
		return lowerBound, upperBound, nil
	}

	if lowerBound.After(lowerBound) {
		return time.Time{}, time.Time{}, ErrLowerBoundAfterUpperBound
	}

	return time.Time{}, time.Time{}, ErrCantDecodePartitionBounds
}

func parseBoundAsDate(partition PartitionResult) (lowerBound, upperBound time.Time, err error) {
	lowerBound, err = time.ParseInLocation("2006-01-02", partition.LowerBound, time.UTC)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("can't parse lowerbound as date: %w", err)
	}

	upperBound, err = time.ParseInLocation("2006-01-02", partition.UpperBound, time.UTC)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("can't parse upperbound as date: %w", err)
	}

	return lowerBound, upperBound, nil
}

func parseBoundAsDateTime(partition PartitionResult) (lowerBound, upperBound time.Time, err error) {
	lowerBound, err = time.ParseInLocation("2006-01-02 15:04:05", partition.LowerBound, time.UTC)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("can't parse lowerbound as datetime: %w", err)
	}

	upperBound, err = time.ParseInLocation("2006-01-02 15:04:05", partition.UpperBound, time.UTC)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("can't parse upperbound as datetime: %w", err)
	}

	return lowerBound, upperBound, nil
}

func parseBoundAsDateTimeWithTimezone(partition PartitionResult) (lowerBound, upperBound time.Time, err error) {
	lowerBound, err = time.ParseInLocation("2006-01-02 15:04:05Z07", partition.LowerBound, time.UTC)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("can't parse lowerbound as datetime with timezone: %w", err)
	}

	upperBound, err = time.ParseInLocation("2006-01-02 15:04:05Z07", partition.UpperBound, time.UTC)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("can't parse upperbound as datetime with timezone: %w", err)
	}

	lowerBound = convertToDateTimeWithoutTimezone(lowerBound)
	upperBound = convertToDateTimeWithoutTimezone(upperBound)

	return lowerBound, upperBound, nil
}

func convertToDateTimeWithoutTimezone(bound time.Time) time.Time {
	parsedTime, err := time.Parse("2006-01-02 15:04:05", bound.UTC().Format("2006-01-02 15:04:05"))
	if err != nil {
		return time.Time{}
	}

	return parsedTime
}
