-- name: AddCall :one
INSERT INTO calls (
	id,
	submitter,
	system,
	talkgroup,
	date,
	audio_name,
	audio_blob,
	audio_type,
	audio_url,
	frequency,
	frequencies,
	patches,
	tg_label,
	source
	) VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13) 
RETURNING *;

-- name: UpdateCallTranscript :exec
UPDATE calls SET transcript = $2 WHERE id = $1;
