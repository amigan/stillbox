package database

import (
	"context"
	"errors"
	"iter"
	"strings"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const fsckTempTableName = "audio_ref_fsck"

type callsQuerier interface {
	GetCallAudioCount(ctx context.Context, arg GetCallAudioParams) (int64, error)

	// GetAudioRefJournalCb calls a callback for each row. Until it returns, the connection is busy.
	GetAudioRefJournalCb(ctx context.Context, arg GetRefJournalParams, sendEntry func(AudioRefJournal)) error

	// GetCallAudioCb calls a callback for each row. Until it returns, the connection is busy.
	GetCallAudioCb(ctx context.Context, arg GetCallAudioParams, cb func(*GetCallAudioRow) error) error

	// CopyIntoFsckTempTable copies from source into table.
	// If progressChan is not nil, it will receive progress of the copy. The channel will be closed on completion.
	CopyIntoFsckTempTable(ctx context.Context, tableName string, ids iter.Seq[uuid.UUID], progressChan chan<- int64) (copied int64, err error)

	// FsckRefs marks as dangling refs from the specified backend that did not appear in the temporary table.
	FsckRefs(ctx context.Context, tableName, backend string) (callsDangling int64, err error)

	// CreateFsckTempTable generates a table name and creates it.
	CreateFsckTempTable(ctx context.Context) (tableName string, err error)

	// DropFsckTempTable drops table "audio_ref_fsck"
	DropFsckTempTable(ctx context.Context, tableName string) error
}

// GetCallAudioCount is here because the following query crashes sqlc.
// I think the bug is https://github.com/sqlc-dev/sqlc/issues/3942
// but maybe not. When it is fixed, we can move this to sqlc:
/*
-- name: GetCallAudioCount :one
SELECT COUNT(*) FROM (
SELECT
	c.id
FROM calls c
JOIN talkgroups tgs ON c.talkgroup = tgs.tgid AND c.system = tgs.system_id
WHERE
CASE WHEN sqlc.narg('swept')::BOOLEAN = TRUE THEN
	FALSE ELSE TRUE END AND
CASE WHEN sqlc.narg('start')::TIMESTAMPTZ IS NOT NULL THEN
	c.call_date >= @start ELSE TRUE END AND
CASE WHEN sqlc.narg('end')::TIMESTAMPTZ IS NOT NULL THEN
	c.call_date <= sqlc.narg('end') ELSE TRUE END AND
CASE WHEN sqlc.narg('tags_any')::TEXT[] IS NOT NULL THEN
	tgs.tags && ARRAY[@tags_any] ELSE TRUE END AND
CASE WHEN sqlc.narg('tags_not')::TEXT[] IS NOT NULL THEN
	(NOT (tgs.tags && ARRAY[@tags_not])) ELSE TRUE END AND
CASE WHEN sqlc.narg('longer_than')::NUMERIC IS NOT NULL THEN (
		c.duration > @longer_than
	) ELSE TRUE END AND
CASE WHEN sqlc.narg('has_backend')::TEXT IS NOT NULL THEN (
	c.audio_ref IS NOT NULL AND c.audio_ref ? @has_backend) ELSE TRUE END AND
CASE WHEN sqlc.narg('not_has_backend')::TEXT IS NOT NULL THEN (
	c.audio_ref IS NULL OR (NOT c.audio_ref ? @not_has_backend)) ELSE TRUE END
AND CASE
	WHEN sqlc.narg('has_blob')::BOOLEAN IS NULL THEN TRUE
	WHEN @has_blob::BOOLEAN = TRUE THEN (c.audio_blob IS NOT NULL)
	WHEN @has_blob::BOOLEAN = FALSE THEN (c.audio_blob IS NULL)
END
UNION
SELECT
	sc.id
FROM swept_calls sc
JOIN talkgroups tgs ON sc.talkgroup = tgs.tgid AND sc.system = tgs.system_id
LEFT JOIN incidents_calls ic ON sc.id = ic.calls_tbl_id AND sc.call_date = ic.call_date
WHERE
CASE WHEN sqlc.narg('swept')::BOOLEAN = FALSE THEN
	FALSE ELSE TRUE END AND
CASE WHEN sqlc.narg('start')::TIMESTAMPTZ IS NOT NULL THEN
	sc.call_date >= @start ELSE TRUE END AND
CASE WHEN sqlc.narg('end')::TIMESTAMPTZ IS NOT NULL THEN
	sc.call_date <= sqlc.narg('end') ELSE TRUE END AND
CASE WHEN sqlc.narg('tags_any')::TEXT[] IS NOT NULL THEN
	tgs.tags && ARRAY[@tags_any] ELSE TRUE END AND
CASE WHEN sqlc.narg('tags_not')::TEXT[] IS NOT NULL THEN
	(NOT (tgs.tags && ARRAY[@tags_not])) ELSE TRUE END AND
CASE WHEN sqlc.narg('longer_than')::NUMERIC IS NOT NULL THEN (
		sc.duration > @longer_than
	) ELSE TRUE END AND
CASE WHEN sqlc.narg('has_backend')::TEXT IS NOT NULL THEN (
	sc.audio_ref IS NOT NULL AND sc.audio_ref ? @has_backend) ELSE TRUE END AND
CASE WHEN sqlc.narg('not_has_backend')::TEXT IS NOT NULL THEN (
	sc.audio_ref IS NULL OR (NOT sc.audio_ref ? @not_has_backend)) ELSE TRUE END
AND CASE
	WHEN @has_blob::BOOLEAN IS NULL THEN TRUE
	WHEN @has_blob::BOOLEAN = TRUE THEN (sc.audio_blob IS NOT NULL)
	WHEN @has_blob::BOOLEAN = FALSE THEN (sc.audio_blob IS NULL)
END
)
;
*/

const getCallAudioCount = `-- name: GetCallAudioCount :one
SELECT COUNT(*) FROM (
SELECT
	c.id
FROM calls c
JOIN talkgroups tgs ON c.talkgroup = tgs.tgid AND c.system = tgs.system_id
LEFT JOIN incidents_calls ic ON c.id = ic.calls_tbl_id AND c.call_date = ic.call_date
WHERE
CASE WHEN $1::BOOLEAN = TRUE THEN
	FALSE ELSE TRUE END AND
CASE WHEN $2::TIMESTAMPTZ IS NOT NULL THEN
	c.call_date >= $2 ELSE TRUE END AND
CASE WHEN $3::TIMESTAMPTZ IS NOT NULL THEN
	c.call_date <= $3 ELSE TRUE END AND
CASE WHEN $4::TEXT[] IS NOT NULL THEN
	tgs.tags && ARRAY[$4] ELSE TRUE END AND
CASE WHEN $5::TEXT[] IS NOT NULL THEN
	(NOT (tgs.tags && ARRAY[$5])) ELSE TRUE END AND
CASE WHEN $6::NUMERIC IS NOT NULL THEN (
		c.duration > $6
	) ELSE TRUE END AND
CASE WHEN $7::TEXT IS NOT NULL THEN (
	c.audio_ref IS NOT NULL AND c.audio_ref ? $7) ELSE TRUE END AND
CASE WHEN $8::TEXT IS NOT NULL THEN (
	c.audio_ref IS NULL OR (NOT c.audio_ref ? $8)) ELSE TRUE END
AND CASE
	WHEN $9::BOOLEAN IS NULL THEN TRUE
	WHEN $9::BOOLEAN = TRUE THEN (c.audio_blob IS NOT NULL)
	WHEN $9::BOOLEAN = FALSE THEN (c.audio_blob IS NULL)
END
UNION
SELECT
	sc.id
FROM swept_calls sc
JOIN talkgroups tgs ON sc.talkgroup = tgs.tgid AND sc.system = tgs.system_id
LEFT JOIN incidents_calls ic ON sc.id = ic.calls_tbl_id AND sc.call_date = ic.call_date
WHERE 
CASE WHEN $1::BOOLEAN = FALSE THEN
	FALSE ELSE TRUE END AND
CASE WHEN $2::TIMESTAMPTZ IS NOT NULL THEN
	sc.call_date >= $2 ELSE TRUE END AND
CASE WHEN $3::TIMESTAMPTZ IS NOT NULL THEN
	sc.call_date <= $3 ELSE TRUE END AND
CASE WHEN $4::TEXT[] IS NOT NULL THEN
	tgs.tags && ARRAY[$4] ELSE TRUE END AND
CASE WHEN $5::TEXT[] IS NOT NULL THEN
	(NOT (tgs.tags && ARRAY[$5])) ELSE TRUE END AND
CASE WHEN $6::NUMERIC IS NOT NULL THEN (
		sc.duration > $6
	) ELSE TRUE END AND
CASE WHEN $7::TEXT IS NOT NULL THEN (
	sc.audio_ref IS NOT NULL AND sc.audio_ref ? $7) ELSE TRUE END AND
CASE WHEN $8::TEXT IS NOT NULL THEN (
	sc.audio_ref IS NULL OR (NOT sc.audio_ref ? $8)) ELSE TRUE END
AND CASE
	WHEN $9::BOOLEAN IS NULL THEN TRUE
	WHEN $9::BOOLEAN = TRUE THEN (sc.audio_blob IS NOT NULL)
	WHEN $9::BOOLEAN = FALSE THEN (sc.audio_blob IS NULL)
END
)
`

func (q *Queries) GetCallAudioCount(ctx context.Context, arg GetCallAudioParams) (int64, error) {
	row := q.db.QueryRow(ctx, getCallAudioCount,
		arg.Swept,
		arg.Start,
		arg.End,
		arg.TagsAny,
		arg.TagsNot,
		arg.LongerThan,
		arg.HasBackend,
		arg.NotHasBackend,
		arg.HasBlob,
	)
	var count int64
	err := row.Scan(&count)
	return count, err
}

func (q *Queries) GetCallAudioCb(ctx context.Context, arg GetCallAudioParams, cb func(*GetCallAudioRow) error) error {
	rows, err := q.db.Query(ctx, getCallAudio,
		arg.Count,
		arg.Swept,
		arg.Start,
		arg.End,
		arg.TagsAny,
		arg.TagsNot,
		arg.LongerThan,
		arg.HasBackend,
		arg.NotHasBackend,
		arg.HasBlob,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var i GetCallAudioRow
		if err := rows.Scan(
			&i.ID,
			&i.CallDate,
			&i.AudioName,
			&i.AudioType,
			&i.AudioRef,
			&i.AudioBlob,
			&i.Swept,
		); err != nil {
			return err
		}
		err = cb(&i)
		if err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return nil
}

func (q *Queries) GetAudioRefJournalCb(ctx context.Context, arg GetRefJournalParams, cb func(AudioRefJournal)) error {
	rows, err := q.db.Query(ctx, getRefJournal,
		arg.Missing,
		arg.Since,
		arg.Until,
		arg.Num,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var i AudioRefJournal
		if err := rows.Scan(
			&i.ID,
			&i.CallID,
			&i.Backend,
			&i.Ref,
			&i.PruneAfter,
			&i.LastTry,
			&i.Tries,
		); err != nil {
			return err
		}
		cb(i)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return nil
}

type CopyFromer interface {
	CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
}

type CopyFromUUIDIter struct {
	next         func() (uuid.UUID, bool)
	stop         func()
	nextVal      uuid.UUID
	count        int64
	progressChan chan<- int64
	noMore       bool
}

const progressStep = 100

func (cfi *CopyFromUUIDIter) Next() bool {
	if cfi.noMore {
		return false
	}

	nextVal, more := cfi.next()
	cfi.nextVal = nextVal
	cfi.noMore = !more
	cfi.count++

	if cfi.progressChan != nil {
		if cfi.count%progressStep == 0 || !more {
			cfi.progressChan <- cfi.count
		}
	}

	return more
}

func (cfi *CopyFromUUIDIter) Close() {
	if cfi.progressChan != nil {
		close(cfi.progressChan)
	}
}

func (cfi *CopyFromUUIDIter) Err() error {
	return nil
}

func (cfi *CopyFromUUIDIter) Stop() {
	cfi.noMore = true
	cfi.stop()
}

func (cfi *CopyFromUUIDIter) Values() ([]any, error) {
	return []any{pgtype.UUID{Bytes: cfi.nextVal, Valid: true}}, cfi.Err()
}

func NewCopyFromUUIDIter(it iter.Seq[uuid.UUID], progressChan chan<- int64) *CopyFromUUIDIter {
	next, stop := iter.Pull(it)
	return &CopyFromUUIDIter{
		next:         next,
		stop:         stop,
		noMore:       false,
		progressChan: progressChan,
	}
}

func (q *Queries) CreateFsckTempTable(ctx context.Context) (tableName string, err error) {
	tName := fsckTempTableName + "_" + common.RandSeq(5)
	_, err = q.db.Exec(ctx, "CREATE TEMPORARY TABLE "+tName+"(id UUID PRIMARY KEY);")

	return tName, err
}

func (q *Queries) CopyIntoFsckTempTable(ctx context.Context, tableName string, ids iter.Seq[uuid.UUID], progressChan chan<- int64) (int64, error) {
	cf, isCf := q.db.(CopyFromer)
	if !isCf {
		return 0, errors.New("querier does not support CopyFrom")
	}

	src := NewCopyFromUUIDIter(ids, progressChan)
	defer src.Close()

	return cf.CopyFrom(ctx, []string{tableName}, []string{"id"}, src)
}

const callsDangleUpdate = `
UPDATE calls
SET dangling_at = $1
WHERE id IN (SELECT id
	FROM calls c
	LEFT JOIN ` + fsckTempTableName + ` th USING (id)
	WHERE c.audio_ref->>$2 IS NOT NULL AND th.id IS NULL);`

const sweptCallsDangleUpdate = `
UPDATE swept_calls
SET dangling_at = $1
WHERE id IN (SELECT id
	FROM swept_calls c
	LEFT JOIN ` + fsckTempTableName + ` th USING (id)
	WHERE c.audio_ref->>$2 IS NOT NULL AND th.id IS NULL);`

func subTableName(s, tblName string) string {
	return strings.ReplaceAll(s, fsckTempTableName, tblName)
}

func (q *Queries) FsckRefs(ctx context.Context, tableName, backend string) (callsDangling int64, err error) {
	fsckBatch := pgtype.Timestamp{Time: time.Now().UTC(), Valid: true}

	ct, err := q.db.Exec(ctx, subTableName(callsDangleUpdate, tableName), fsckBatch, backend)
	if err != nil {
		return
	}
	callsDangling += ct.RowsAffected()

	ct, err = q.db.Exec(ctx, subTableName(sweptCallsDangleUpdate, tableName), fsckBatch, backend)
	if err != nil {
		return
	}
	callsDangling += ct.RowsAffected()

	return
}

func (q *Queries) DropFsckTempTable(ctx context.Context, tableName string) error {
	_, err := q.db.Exec(ctx, "DROP TABLE "+tableName+";")
	return err
}
