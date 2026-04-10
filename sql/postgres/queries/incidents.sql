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
ON CONFLICT DO NOTHING
;

-- name: RemoveFromIncident :exec
DELETE FROM incidents_calls ic
WHERE ic.incident_id = @id AND ic.call_id = ANY(@call_ids::UUID[]);

-- name: UpdateCallIncidentNotes :exec
UPDATE incidents_Calls
SET notes = @notes
WHERE incident_id = @incident_id AND call_id = @call_id;

-- name: CallInIncident :one
SELECT EXISTS
	(SELECT 1 FROM incidents_calls ic
	WHERE
	ic.incident_id = @incident_id AND
	ic.call_id = @call_id);

-- name: CreateIncident :one
INSERT INTO incidents (
	id,
	name,
	owner_id,
	description,
	created_at,
	start_time,
	end_time,
	location,
	metadata
) VALUES (
	@id,
	@name,
	@owner_id,
	sqlc.narg('description'),
	NOW(),
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
	i.owner_id,
	i.description,
	i.created_at,
	COALESCE(i.start_time, MIN(ic.call_date)) start_time,
	COALESCE(i.end_time, MAX(ic.call_date)) end_time,
	i.location,
	i.metadata,
	u.username owner,
	COUNT(ic.incident_id) calls_count
FROM incidents i
LEFT JOIN incidents_calls ic ON i.id = ic.incident_id
JOIN users u ON i.owner_id = u.id
WHERE
CASE WHEN sqlc.narg('start')::TIMESTAMPTZ IS NOT NULL THEN
	i.start_time >= sqlc.narg('start') ELSE TRUE END AND
CASE WHEN sqlc.narg('end')::TIMESTAMPTZ IS NOT NULL THEN
	i.start_time <= sqlc.narg('end') ELSE TRUE END AND
(CASE WHEN sqlc.narg('filter')::TEXT IS NOT NULL THEN (
		i.name ILIKE '%' || @filter || '%' OR
		i.description ILIKE '%' || @filter || '%'
	) ELSE TRUE END)
GROUP BY i.id, u.username
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
	i.start_time <= sqlc.narg('end') ELSE TRUE END AND
(CASE WHEN sqlc.narg('filter')::TEXT IS NOT NULL THEN (
		i.name ILIKE '%' || @filter || '%' OR
		i.description ILIKE '%' || @filter || '%'
	) ELSE TRUE END)
;

-- name: GetIncidentCallCount :one
SELECT COUNT(*) FROM incidents_calls
WHERE incident_id = @incident_id;

-- name: GetIncidentCalls :many
SELECT
	ic.call_id,
	ic.call_date,
	c.duration,
	c.system system_id,
	c.talkgroup tgid,
	ic.notes,
	c.submitter,
	c.audio_name,
	c.audio_type,
	c.audio_ref,
	c.frequency,
	c.frequencies,
	c.talker_alias,
	c.patches,
	c.source,
	c.transcript
FROM incidents_calls ic, LATERAL (
	SELECT 
	ca.submitter,
	ca.system,
	ca.talkgroup,
	ca.audio_name,
	ca.duration,
	ca.audio_type,
	ca.audio_ref,
	ca.frequency,
	ca.frequencies,
	ca.talker_alias,
	ca.patches,
	ca.source,
	ca.transcript
	FROM calls ca WHERE ca.id = ic.calls_tbl_id AND ca.call_date = ic.call_date
	UNION
	SELECT
	sc.submitter,
	sc.system,
	sc.talkgroup,
	sc.audio_name,
	sc.duration,
	sc.audio_type,
	sc.audio_ref,
	sc.frequency,
	sc.frequencies,
	sc.talker_alias,
	sc.patches,
	sc.source,
	sc.transcript
	FROM swept_calls sc WHERE sc.id = ic.swept_call_id
) c
WHERE ic.incident_id = @id
ORDER BY ic.call_date ASC
OFFSET sqlc.arg('offset') ROWS
FETCH NEXT sqlc.narg('per_page') ROWS ONLY
;

-- name: GetIncident :one
SELECT
	sqlc.embed(incidents),
	COALESCE(incidents.start_time, MIN(ic.call_date)) start_time,
	COALESCE(incidents.end_time, MAX(ic.call_date)) end_time,
	users.username owner
FROM incidents
LEFT JOIN incidents_calls ic ON incidents.id = ic.incident_id
JOIN users ON incidents.owner_id = users.id
WHERE incidents.id = @id
GROUP BY incidents.id, users.username;

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

-- name: GetIncidentOwner :one
SELECT owner_id FROM incidents WHERE id = @id;
