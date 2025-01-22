-- name: GetShare :one
SELECT
	id,
	entity_type,
	entity_id,
	entity_date,
	owner,
	expiration
FROM shares
WHERE id = @id;

-- name: CreateShare :exec
INSERT INTO shares (
	id,
	entity_type,
	entity_id,
	entity_date,
	owner,
	expiration
) VALUES (@id, @entity_type, @entity_id, sqlc.narg('entity_date'), @owner, sqlc.narg('expiration'));

-- name: DeleteShare :exec
DELETE FROM shares WHERE id = @id;

-- name: PruneShares :exec
DELETE FROM shares WHERE expiration < NOW();

-- name: GetIncidentTalkgroups :many
SELECT DISTINCT
	c.system,
	c.talkgroup
FROM incidents_calls ic 
JOIN calls c ON (c.id = ic.call_id AND c.call_date = ic.call_date) 
WHERE ic.incident_id = @incident_id;
