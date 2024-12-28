-- name: AddToIncident :exec
WITH inp AS (
SELECT
	UNNEST(@call_ids::UUID[]) id,
	UNNEST(@notes::JSONB[]) notes
) INSERT INTO incidents_calls(
	incident_id,
	call_id,
	calls_tbl_id,
	call_date,
	notes
)
SELECT
	@incident_id::UUID,
	inp.id,
	inp.id,
	c.call_date,
	inp.notes
FROM inp
JOIN calls c ON c.id = inp.id
;

-- name: CreateIncident :one
INSERT INTO incidents (
	id,
	name,
	description,
	start_time,
	end_time,
	location,
	metadata
) VALUES (
	@id,
	@name,
	sqlc.narg('description'),
	sqlc.narg('start_time'),
	sqlc.narg('end_time'),
	sqlc.narg('location'),
	sqlc.narg('metadata')
)
RETURNING *;


-- name: ListIncidentsP :many
SELECT
	i.id,
	i.name,
	i.description,
	i.start_time,
	i.end_time,
	i.location,
	i.metadata
FROM incidents i
WHERE
CASE WHEN sqlc.narg('start')::TIMESTAMPTZ IS NOT NULL THEN
	i.start_time >= sqlc.narg('start') ELSE TRUE END AND
CASE WHEN sqlc.narg('end')::TIMESTAMPTZ IS NOT NULL THEN
	i.start_time <= sqlc.narg('end') ELSE TRUE END
ORDER BY
CASE WHEN @direction::TEXT = 'asc' THEN i.start_time END ASC,
CASE WHEN @direction::TEXT = 'desc' THEN i.start_time END DESC
OFFSET sqlc.arg('offset') ROWS
FETCH NEXT sqlc.arg('per_page') ROWS ONLY
;

-- name: ListIncidentsCount :one
SELECT COUNT(*)
FROM incidents i
WHERE
CASE WHEN sqlc.narg('start')::TIMESTAMPTZ IS NOT NULL THEN
	i.start_time >= sqlc.narg('start') ELSE TRUE END AND
CASE WHEN sqlc.narg('end')::TIMESTAMPTZ IS NOT NULL THEN
	i.start_time <= sqlc.narg('end') ELSE TRUE END
;

-- name: IncidentCalls :many
-- INCOMPLETE
SELECT
ic.incident_id, call_date,
ic.call_id,
ic.notes
FROM incidents_calls ic;

-- name: GetIncident :one
SELECT
	i.id,
	i.name,
	i.description,
	i.start_time,
	i.end_time,
	i.location,
	i.metadata
FROM incidents i
WHERE i.id = @id;

-- name: UpdateIncident :one
UPDATE incidents
SET
	name = COALESCE(sqlc.narg('name'), name),
	description = COALESCE(sqlc.narg('description'), description),
	start_time = COALESCE(sqlc.narg('start_time'), start_time),
	end_time = COALESCE(sqlc.narg('end_time'), end_time),
	location = COALESCE(sqlc.narg('location'), location),
	metadata = COALESCE(sqlc.narg('metadata'), metadata)
WHERE
	id = @id
RETURNING *;

-- name: DeleteIncident :exec
DELETE FROM incidents CASCADE WHERE id = @id;
