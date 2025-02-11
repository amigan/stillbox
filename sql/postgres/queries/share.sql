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

-- name: GetSharesP :many
SELECT
	s.id,
	s.entity_type,
	s.entity_id,
	s.entity_date,
	s.owner,
	s.expiration
FROM shares s
WHERE
CASE WHEN sqlc.narg('owner')::INTEGER IS NOT NULL THEN
	s.owner = @owner ELSE TRUE END
ORDER BY
CASE WHEN @direction::TEXT = 'asc' THEN s.entity_date END ASC,
CASE WHEN @direction::TEXT = 'desc' THEN s.entity_date END DESC
OFFSET sqlc.arg('offset') ROWS
FETCH NEXT sqlc.arg('per_page') ROWS ONLY
;

-- name: GetSharesPCount :one
SELECT COUNT(*)
FROM shares s
WHERE
CASE WHEN sqlc.narg('owner')::INTEGER IS NOT NULL THEN
	s.owner = @owner ELSE TRUE END
;
