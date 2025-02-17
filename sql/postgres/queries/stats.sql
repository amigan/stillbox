-- name: GetCallStatsByTalkgroup :many
SELECT
	COUNT(*),
	c.system,
	c.talkgroup,
	date_trunc(@trunc_field, c.call_date)
FROM calls c
WHERE
CASE WHEN sqlc.narg('start')::TIMESTAMPTZ IS NOT NULL THEN
	c.call_date >= @start ELSE TRUE END AND
CASE WHEN sqlc.narg('end')::TIMESTAMPTZ IS NOT NULL THEN
	c.call_date <= sqlc.narg('end') ELSE TRUE END
GROUP BY 2, 3, 4
ORDER BY 4 DESC;

-- name: GetCallStatsByInterval :many
SELECT
	COUNT(*),
	date_trunc(@trunc_field, c.call_date)::TIMESTAMPTZ date
FROM calls c
WHERE
CASE WHEN sqlc.narg('start')::TIMESTAMPTZ IS NOT NULL THEN
	c.call_date >= @start ELSE TRUE END AND
CASE WHEN sqlc.narg('end')::TIMESTAMPTZ IS NOT NULL THEN
	c.call_date <= sqlc.narg('end') ELSE TRUE END
GROUP BY date
ORDER BY date DESC;
