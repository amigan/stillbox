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
	frequency,
	frequencies,
	patches,
	tg_label,
	tg_tag,
	tg_group,
	source
	) VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15) 
RETURNING *;

-- name: SetCallTranscript :exec
UPDATE calls SET transcript = $2 WHERE id = $1;
