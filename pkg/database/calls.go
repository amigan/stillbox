package database

import (
	"context"
)

type callsQuerier interface {
	GetCallAudioCount(ctx context.Context, arg GetCallAudioParams) (int64, error)
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
	c.audio_ref ? @has_backend) ELSE TRUE END AND
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
	sc.audio_ref ? @has_backend) ELSE TRUE END AND
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
	NOT (c.audio_ref ? $8)) ELSE TRUE END AND
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
	sc.audio_ref ? $7) ELSE TRUE END AND
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
