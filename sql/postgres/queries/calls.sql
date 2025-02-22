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
talker_alias,
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
@talker_alias,
@tg_label,
@tg_alpha_tag,
@tg_group,
@source
);

-- name: GetCallAudioByID :one
SELECT
	c.call_date,
	c.audio_name,
	c.audio_type,
	c.audio_blob
FROM calls c
WHERE c.id = @id
UNION
SELECT
	sc.call_date,
	sc.audio_name,
	sc.audio_type,
	sc.audio_blob
FROM swept_calls sc
WHERE sc.id = @id
;

-- name: SetCallTranscript :exec
UPDATE calls SET transcript = $2 WHERE id = $1;

-- name: AddAlert :exec
INSERT INTO alerts (time, tgid, system_id, weight, score, orig_score, notified, metadata)
VALUES
(
	sqlc.arg(time),
	sqlc.arg(tgid),
	sqlc.arg(system_id),
	sqlc.arg(weight),
	sqlc.arg(score),
	sqlc.arg(orig_score),
	sqlc.arg(notified),
	sqlc.arg(metadata)
);

-- name: GetDatabaseSize :one
SELECT pg_size_pretty(pg_database_size(current_database()));

-- name: SweepCalls :execrows
-- This is used to sweep calls that are part of an incident prior to pruning a partition.
WITH to_sweep AS (
	SELECT id, submitter, system, talkgroup, calls.call_date, audio_name, audio_blob, duration, audio_type,
		audio_url, frequency, frequencies, talker_alias, patches, tg_label, tg_alpha_tag, tg_group, source, transcript
	FROM calls
	JOIN incidents_calls ic ON ic.call_id = calls.id
	WHERE calls.call_date >= @range_start AND calls.call_date < @range_end
) INSERT INTO swept_calls SELECT * FROM to_sweep;

-- name: CleanupSweptCalls :execrows
WITH to_sweep AS (
	SELECT id FROM calls
	JOIN incidents_calls ic ON ic.call_id = calls.id
	WHERE calls.call_date >= @range_start AND calls.call_date < @range_end
) UPDATE incidents_calls
	SET
		swept_call_id = call_id,
		calls_tbl_id = NULL
	WHERE call_id IN (SELECT id FROM to_sweep);

-- name: ListCallsP :many
SELECT
c.id,
c.call_date,
c.duration,
c.system system_id,
c.talkgroup tgid,
c.talker_alias,
COUNT(ic.incident_id) incidents
FROM calls c
JOIN talkgroups tgs ON c.talkgroup = tgs.tgid AND c.system = tgs.system_id
JOIN systems sys ON sys.id = tgs.system_id
LEFT JOIN incidents_calls ic ON c.id = ic.calls_tbl_id AND c.call_date = ic.call_date
WHERE
CASE WHEN sqlc.narg('start')::TIMESTAMPTZ IS NOT NULL THEN
	c.call_date >= @start ELSE TRUE END AND
CASE WHEN sqlc.narg('end')::TIMESTAMPTZ IS NOT NULL THEN
	c.call_date <= sqlc.narg('end') ELSE TRUE END AND
CASE WHEN sqlc.narg('tags_any')::TEXT[] IS NOT NULL THEN
	tgs.tags && ARRAY[@tags_any] ELSE TRUE END AND
CASE WHEN sqlc.narg('tags_not')::TEXT[] IS NOT NULL THEN
	(NOT (tgs.tags && ARRAY[@tags_not])) ELSE TRUE END AND
(CASE WHEN sqlc.narg('tg_filter')::TEXT IS NOT NULL THEN (
		tgs.tg_group ILIKE '%' || @tg_filter || '%' OR
		tgs.name ILIKE '%' || @tg_filter || '%' OR
		tgs.alpha_tag ILIKE '%' || @tg_filter || '%'
	) ELSE TRUE END) AND
(CASE WHEN sqlc.narg('longer_than')::NUMERIC IS NOT NULL THEN (
		c.duration > @longer_than
	) ELSE TRUE END) AND
(CASE WHEN @unknown_tg::BOOLEAN = TRUE THEN (
	tgs.tgid IS NULL
	) ELSE TRUE END)
GROUP BY c.id, c.call_date
ORDER BY
CASE WHEN @direction::TEXT = 'asc' THEN c.call_date END ASC,
CASE WHEN @direction = 'desc' THEN c.call_date END DESC
OFFSET sqlc.arg('offset') ROWS
FETCH NEXT sqlc.arg('per_page') ROWS ONLY
;

-- name: ListCallsCount :one
SELECT
COUNT(*)
FROM calls c
JOIN talkgroups tgs ON c.talkgroup = tgs.tgid AND c.system = tgs.system_id
WHERE
CASE WHEN sqlc.narg('start')::TIMESTAMPTZ IS NOT NULL THEN
	c.call_date >= @start ELSE TRUE END AND
CASE WHEN sqlc.narg('end')::TIMESTAMPTZ IS NOT NULL THEN
	c.call_date <= sqlc.narg('end') ELSE TRUE END AND
CASE WHEN sqlc.narg('tags_any')::TEXT[] IS NOT NULL THEN
	tgs.tags && ARRAY[@tags_any] ELSE TRUE END AND
CASE WHEN sqlc.narg('tags_not')::TEXT[] IS NOT NULL THEN
	(NOT (tgs.tags && ARRAY[@tags_not])) ELSE TRUE END AND
(CASE WHEN sqlc.narg('tg_filter')::TEXT IS NOT NULL THEN (
		tgs.tg_group ILIKE '%' || @tg_filter || '%' OR
		tgs.name ILIKE '%' || @tg_filter || '%' OR
		tgs.alpha_tag ILIKE '%' || @tg_filter || '%'
	) ELSE TRUE END) AND
(CASE WHEN sqlc.narg('longer_than')::NUMERIC IS NOT NULL THEN (
		c.duration > @longer_than
	) ELSE TRUE END) AND
(CASE WHEN @unknown_tg::BOOLEAN = TRUE THEN (
	tgs.tgid IS NULL
	) ELSE TRUE END)
;

-- name: DeleteCall :exec
DELETE FROM calls WHERE id = @id;

-- name: GetCallSubmitter :one
SELECT submitter FROM calls WHERE id = @id;

-- name: GetCall :one
SELECT
	id,
	submitter,
	system,
	talkgroup,
	call_date,
	audio_name,
	audio_type,
	audio_url,
	duration,
	frequency,
	frequencies,
	patches,
	talker_alias,
	tg_label,
	tg_alpha_tag,
	tg_group,
	source,
	transcript
FROM calls
WHERE id = @id;
