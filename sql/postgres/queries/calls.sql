-- name: AddCall :exec
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
) VALUES (
@id,
@submitter,
@system,
@talkgroup,
@call_date,
@audio_name,
@audio_blob,
@audio_type,
@audio_url,
@duration,
@frequency,
@frequencies,
@patches,
@tg_label,
@tg_alpha_tag,
@tg_group,
@source
);

-- name: SetCallTranscript :exec
UPDATE calls SET transcript = $2 WHERE id = $1;

-- name: AddAlert :exec
INSERT INTO alerts (id, time, talkgroup, weight, score, orig_score, notified, metadata)
VALUES
(
	sqlc.arg(id),
	sqlc.arg(time),
	sqlc.arg(packed_tg),
	sqlc.arg(weight),
	sqlc.arg(score),
	sqlc.arg(orig_score),
	sqlc.arg(notified),
	sqlc.arg(metadata)
);

-- name: GetDatabaseSize :one
SELECT pg_size_pretty(pg_database_size(current_database()));
