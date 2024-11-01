-- name: AddCall :one
INSERT INTO calls (
	id,
	submitter,
	system,
	talkgroup,
	call_date,
	audio_name,
	audio_blob,
	audio_type,
	audio_url,
	duration,
	frequency,
	frequencies,
	patches,
	tg_label,
	tg_alpha_tag,
	tg_group,
	source
	) VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16) 
RETURNING id;

-- name: SetCallTranscript :exec
UPDATE calls SET transcript = $2 WHERE id = $1;

-- name: AddAlert :exec
INSERT INTO alerts (id, time, talkgroup, weight, score, metadata)
VALUES
(
	sqlc.arg(id),
	sqlc.arg(time),
	sqlc.arg(packed_tg),
	sqlc.arg(weight),
	sqlc.arg(score),
	sqlc.arg(metadata)
);

-- name: GetDatabaseSize :one
SELECT pg_size_pretty(pg_database_size(current_database()));
