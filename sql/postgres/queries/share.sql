-- name: GetShare :one
SELECT
	id,
	entity_type,
	entity_id,
	owner,
	expiration
FROM shares
WHERE id = @id;

-- name: CreateShare :exec
INSERT INTO shares (
	id,
	entity_type,
	entity_id,
	owner,
	expiration
) VALUES (@id, @entity_type, @entity_id, @owner, sqlc.narg('expiration'));

-- name: DeleteShare :exec
DELETE FROM shares WHERE id = @id;

-- name: PruneShares :exec
DELETE FROM shares WHERE expiration < NOW();
