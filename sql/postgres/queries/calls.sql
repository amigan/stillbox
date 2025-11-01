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
audio_ref,
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
@audio_ref,
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
	c.audio_ref,
	c.audio_blob
FROM calls c
WHERE c.id = @id
UNION
SELECT
	sc.call_date,
	sc.audio_name,
	sc.audio_type,
	sc.audio_ref,
	sc.audio_blob
FROM swept_calls sc
WHERE sc.id = @id
;

-- name: GetSweptCallsWithRef :many
SELECT id, call_date, audio_ref, audio_blob FROM swept_calls WHERE audio_ref IS NOT NULL;

-- name: SetSweptAudioAndClearRef :exec
UPDATE swept_calls SET audio_blob = @audio_blob, audio_ref = NULL WHERE id = @id;

-- name: GetCallAudio :many
-- For now, this must be kept in sync with pkg/database/calls.go GetCallAudioCount
SELECT
	c.id,
	c.call_date,
	c.audio_name,
	c.audio_type,
	c.audio_ref,
	c.audio_blob,
	FALSE AS swept
FROM calls c
JOIN talkgroups tgs ON c.talkgroup = tgs.tgid AND c.system = tgs.system_id
LEFT JOIN incidents_calls ic ON c.id = ic.calls_tbl_id AND c.call_date = ic.call_date
WHERE
CASE WHEN sqlc.narg('swept')::BOOLEAN = TRUE THEN
	FALSE ELSE TRUE END AND
CASE WHEN sqlc.narg('start')::TIMESTAMPTZ IS NOT NULL THEN
	c.call_date >= @start ELSE TRUE END AND
CASE WHEN sqlc.narg('end')::TIMESTAMPTZ IS NOT NULL THEN
	c.call_date <= sqlc.narg('end') ELSE TRUE END AND
CASE WHEN sqlc.narg('tags_any')::TEXT[] IS NOT NULL THEN
	tgs.tags && ARRAY[@tags_any] ELSE TRUE END AND
CASE WHEN sqlc.narg('tags_not')::TEXT[] IS NOT NULL THEN
	(NOT (tgs.tags && ARRAY[@tags_not])) ELSE TRUE END AND
CASE WHEN sqlc.narg('longer_than')::NUMERIC IS NOT NULL THEN (
		c.duration > @longer_than
	) ELSE TRUE END AND
CASE WHEN sqlc.narg('has_backend')::TEXT IS NOT NULL THEN (
	c.audio_ref IS NOT NULL AND c.audio_ref ? @has_backend) ELSE TRUE END AND
CASE WHEN sqlc.narg('not_has_backend')::TEXT IS NOT NULL THEN (
	c.audio_ref IS NULL OR (NOT c.audio_ref ? @not_has_backend)) ELSE TRUE END
AND CASE
	WHEN sqlc.narg('has_blob')::BOOLEAN IS NULL THEN TRUE
	WHEN @has_blob::BOOLEAN = TRUE THEN (c.audio_blob IS NOT NULL)
	WHEN @has_blob::BOOLEAN = FALSE THEN (c.audio_blob IS NULL)
END
UNION
SELECT
	sc.id,
	sc.call_date,
	sc.audio_name,
	sc.audio_type,
	sc.audio_ref,
	sc.audio_blob,
	TRUE AS swept
FROM swept_calls sc
JOIN talkgroups tgs ON sc.talkgroup = tgs.tgid AND sc.system = tgs.system_id
LEFT JOIN incidents_calls ic ON sc.id = ic.calls_tbl_id AND sc.call_date = ic.call_date
WHERE 
CASE WHEN sqlc.narg('swept')::BOOLEAN = FALSE THEN
	FALSE ELSE TRUE END AND
CASE WHEN sqlc.narg('start')::TIMESTAMPTZ IS NOT NULL THEN
	sc.call_date >= @start ELSE TRUE END AND
CASE WHEN sqlc.narg('end')::TIMESTAMPTZ IS NOT NULL THEN
	sc.call_date <= sqlc.narg('end') ELSE TRUE END AND
CASE WHEN sqlc.narg('tags_any')::TEXT[] IS NOT NULL THEN
	tgs.tags && ARRAY[@tags_any] ELSE TRUE END AND
CASE WHEN sqlc.narg('tags_not')::TEXT[] IS NOT NULL THEN
	(NOT (tgs.tags && ARRAY[@tags_not])) ELSE TRUE END AND
CASE WHEN sqlc.narg('longer_than')::NUMERIC IS NOT NULL THEN (
		sc.duration > @longer_than
	) ELSE TRUE END AND
CASE WHEN sqlc.narg('has_backend')::TEXT IS NOT NULL THEN (
	sc.audio_ref IS NOT NULL AND sc.audio_ref ? @has_backend) ELSE TRUE END AND
CASE WHEN sqlc.narg('not_has_backend')::TEXT IS NOT NULL THEN (
	sc.audio_ref IS NULL OR (NOT sc.audio_ref ? @not_has_backend)) ELSE TRUE END
AND CASE
	WHEN @has_blob::BOOLEAN IS NULL THEN TRUE
	WHEN @has_blob::BOOLEAN = TRUE THEN (sc.audio_blob IS NOT NULL)
	WHEN @has_blob::BOOLEAN = FALSE THEN (sc.audio_blob IS NULL)
END
ORDER BY call_date ASC
FETCH NEXT sqlc.arg('count') ROWS ONLY
;


-- name: SetCallAudio :exec
UPDATE calls SET audio_ref = @audio_ref, audio_blob = @audio_blob
WHERE id = $1;

-- name: SetSweptCallAudio :exec
UPDATE swept_calls SET audio_ref = @audio_ref, audio_blob = @audio_blob
WHERE id = $1;

-- name: SetCallTranscript :one
UPDATE calls SET transcript = $2 WHERE id = $1
RETURNING call_date, system, talkgroup, patches;

-- name: GetDatabaseSize :one
SELECT pg_size_pretty(pg_database_size(current_database()));

-- name: SweepCalls :execrows
-- This is used to sweep calls that are part of an incident prior to pruning a partition.
WITH to_sweep AS (
	SELECT id,
	submitter,
	system,
	talkgroup,
	calls.call_date,
	audio_name,
	audio_blob,
	duration,
	audio_type,
	audio_ref,
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
	JOIN incidents_calls ic ON ic.call_id = calls.id
	WHERE calls.call_date >= @range_start AND calls.call_date < @range_end
) INSERT INTO swept_calls (
	id,
	submitter,
	system,
	talkgroup,
	call_date,
	audio_name,
	audio_blob,
	duration,
	audio_type,
	audio_ref,
	frequency,
	frequencies,
	patches,
	talker_alias,
	tg_label,
	tg_alpha_tag,
	tg_group,
	source,
	transcript
) SELECT 
	id,
	submitter,
	system,
	talkgroup,
	call_date,
	audio_name,
	audio_blob,
	duration,
	audio_type,
	audio_ref,
	frequency,
	frequencies,
	patches,
	talker_alias,
	tg_label,
	tg_alpha_tag,
	tg_group,
	source,
	transcript
FROM to_sweep ON CONFLICT DO NOTHING;

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
c.source,
(CASE
	WHEN sqlc.narg('transcript_search')::TEXT = '' THEN c.transcript
	WHEN @transcript_search IS NOT NULL THEN 
	ts_headline(c.transcript,
		websearch_to_tsquery('english', @transcript_search),
		'HighlightAll=true')
	ELSE NULL END) transcript,
COUNT(ic.incident_id) incidents,
(c.transcript IS NOT NULL)::BOOLEAN has_transcript
FROM calls c
JOIN talkgroups tgs ON c.talkgroup = tgs.tgid AND c.system = tgs.system_id
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
(CASE WHEN sqlc.narg('source_filter')::TEXT IS NOT NULL THEN (
		c.talker_alias ILIKE '%' || @source_filter || '%'
	) ELSE TRUE END) AND
(CASE WHEN sqlc.narg('longer_than')::NUMERIC IS NOT NULL THEN (
		c.duration > @longer_than
	) ELSE TRUE END) AND
(CASE WHEN @unknown_tg::BOOLEAN = TRUE THEN (
	tgs.tgid IS NULL
	) ELSE TRUE END) AND
(CASE WHEN sqlc.narg('transcript_search')::TEXT IS NOT NULL AND @transcript_search != '' THEN (
	to_tsvector('english', c.transcript) @@ websearch_to_tsquery('english', @transcript_search)
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
(CASE WHEN sqlc.narg('source_filter')::TEXT IS NOT NULL THEN (
		c.talker_alias ILIKE '%' || @source_filter || '%'
	) ELSE TRUE END) AND
(CASE WHEN sqlc.narg('longer_than')::NUMERIC IS NOT NULL THEN (
		c.duration > @longer_than
	) ELSE TRUE END) AND
(CASE WHEN @unknown_tg::BOOLEAN = TRUE THEN (
	tgs.tgid IS NULL
	) ELSE TRUE END) AND
(CASE WHEN sqlc.narg('transcript_search')::TEXT IS NOT NULL AND @transcript_search != '' THEN (
	to_tsvector('english', transcript) @@ websearch_to_tsquery('english', @transcript_search)
	) ELSE TRUE END)
;

-- name: DeleteCall :exec
DELETE FROM calls WHERE id = @id;

-- name: GetCallSubmitter :one
SELECT submitter FROM calls WHERE id = @id;

-- name: GetTranscriptsContext :many
SELECT c.call_date, c.transcript FROM calls c
WHERE
(c.system, c.talkgroup) = (@system, @talkgroup) AND
c.call_date >= NOW() - @lookback::interval AND
c.transcript IS NOT NULL AND
c.duration > @duration_ms
ORDER BY c.call_date DESC
LIMIT @num_transcripts;

-- name: GetCall :one
SELECT
	id,
	submitter,
	system,
	talkgroup,
	call_date,
	audio_name,
	audio_type,
	audio_ref,
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

-- name: GetCalls :many
SELECT sqlc.embed(calls) FROM calls WHERE id = ANY(@ids::UUID[]);

-- name: AddRefJournal :one
INSERT INTO audio_ref_journal (
	call_id,
	backend,
	ref,
	prune_after,
	last_try,
	tries
) VALUES (
	sqlc.narg('call_id'),
	@backend,
	@ref,
	@prune_after,
	NOW(),
	@tries
) RETURNING id;

-- name: SetRefJournalPrune :exec
UPDATE audio_ref_journal SET
	prune_after = sqlc.narg('prune_after')
WHERE
	id = $1;

-- name: IncrementRefJournal :exec
UPDATE audio_ref_journal SET
	tries = tries + 1,
	last_try = NOW()
WHERE id = @id;

-- name: DropRefJournal :exec
DELETE FROM audio_ref_journal
WHERE id = @id;

-- name: GetRefJournal :many
SELECT id, call_id, backend, ref, prune_after, last_try, tries
FROM
audio_ref_journal
WHERE
	(prune_after IS NULL OR NOW() > prune_after) AND
	CASE
		WHEN sqlc.narg('missing')::BOOLEAN IS TRUE THEN ref IS NULL
		WHEN @missing::BOOLEAN IS FALSE THEN ref IS NOT NULL
		ELSE TRUE
	END AND
	CASE WHEN sqlc.narg('since')::TIMESTAMPTZ IS NOT NULL THEN last_try > @since ELSE TRUE END AND
	CASE WHEN sqlc.narg('until')::TIMESTAMPTZ IS NOT NULL THEN last_try <= @until ELSE TRUE END
ORDER BY backend, last_try ASC
LIMIT (CASE WHEN sqlc.narg('num')::INTEGER IS NOT NULL THEN @num ELSE 10000000000 END);

-- name: DetailedCountRefJournal :many
SELECT
COUNT(*), backend, (ref IS NULL)::BOOLEAN has_ref
FROM audio_ref_journal
GROUP BY backend, has_ref;

-- name: CountRefJournal :one
SELECT COUNT(*)
FROM
audio_ref_journal
WHERE
	(prune_after IS NULL OR NOW() > prune_after) AND
	CASE
		WHEN sqlc.narg('missing')::BOOLEAN IS TRUE THEN ref IS NULL
		WHEN @missing::BOOLEAN IS FALSE THEN ref IS NOT NULL
		ELSE TRUE
	END AND
	CASE WHEN sqlc.narg('since')::TIMESTAMPTZ IS NOT NULL THEN last_try > @since ELSE TRUE END AND
	CASE WHEN sqlc.narg('until')::TIMESTAMPTZ IS NOT NULL THEN last_try <= @until ELSE TRUE END
;

-- name: GetPrunableAudioRefs :many
SELECT
	r.backend::TEXT backend,
	LEFT(r.ref, POSITION('/' IN r.ref)) path_first
FROM
	calls
CROSS JOIN
	jsonb_each_text(audio_ref) AS r (backend, ref)
WHERE
	call_date > @partition_start AND call_date <= @partition_end
GROUP BY
	r.backend, path_first;
