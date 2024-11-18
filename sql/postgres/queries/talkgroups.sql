-- name: GetTalkgroupsWithAnyTags :many
SELECT sqlc.embed(talkgroups) FROM talkgroups
WHERE tags @> ARRAY[$1];

-- name: GetTalkgroupsWithAllTags :many
SELECT sqlc.embed(talkgroups) FROM talkgroups
WHERE tags && ARRAY[$1];

-- name: GetTalkgroupIDsByTags :many
SELECT system_id, tgid FROM talkgroups
WHERE (tags @> ARRAY[@any_tags])
AND (tags && ARRAY[@all_tags])
AND NOT (tags @> ARRAY[@not_tags]);

-- name: GetTalkgroupTags :one
SELECT tags FROM talkgroups
WHERE system_id = @system_id AND tgid = @tg_id;

-- name: SetTalkgroupTags :exec
UPDATE talkgroups SET tags = @tags
WHERE system_id = @system_id AND tgid = @tg_id;

-- name: GetTalkgroup :one
SELECT sqlc.embed(talkgroups) FROM talkgroups
WHERE (system_id, tgid) = (@system_id, @tg_id);

-- name: GetTalkgroupWithLearned :one
SELECT
sqlc.embed(tg), sqlc.embed(sys)
FROM talkgroups tg
JOIN systems sys ON tg.system_id = sys.id
WHERE (tg.system_id, tg.tgid) = (@system_id, @tgid) AND tg.learned IS NOT TRUE
UNION
SELECT
tgl.id, tgl.system_id::INT4, tgl.tgid::INT4, tgl.name,
tgl.alpha_tag, tgl.tg_group, NULL::INTEGER, NULL::JSONB,
CASE WHEN tgl.tg_group IS NULL THEN NULL ELSE ARRAY[tgl.tg_group] END,
TRUE, NULL::JSONB, 1.0, TRUE learned, sys.id, sys.name
FROM talkgroups_learned tgl
JOIN systems sys ON tgl.system_id = sys.id
WHERE tgl.system_id = @system_id AND tgl.tgid = @tgid AND ignored IS NOT TRUE;

-- name: GetTalkgroupsWithLearnedBySystem :many
SELECT
sqlc.embed(tg), sqlc.embed(sys)
FROM talkgroups tg
JOIN systems sys ON tg.system_id = sys.id
WHERE tg.system_id = @system AND tg.learned IS NOT TRUE
UNION
SELECT
tgl.id, tgl.system_id::INT4, tgl.tgid::INT4, tgl.name,
tgl.alpha_tag, tgl.tg_group, NULL::INTEGER, NULL::JSONB,
CASE WHEN tgl.tg_group IS NULL THEN NULL ELSE ARRAY[tgl.tg_group] END,
TRUE, NULL::JSONB, 1.0, TRUE learned, sys.id, sys.name
FROM talkgroups_learned tgl
JOIN systems sys ON tgl.system_id = sys.id
WHERE tgl.system_id = @system AND ignored IS NOT TRUE;

-- name: GetTalkgroupsWithLearned :many
SELECT
sqlc.embed(tg), sqlc.embed(sys)
FROM talkgroups tg
JOIN systems sys ON tg.system_id = sys.id
WHERE tg.learned IS NOT TRUE
UNION
SELECT
tgl.id, tgl.system_id::INT4, tgl.tgid::INT4, tgl.name,
tgl.alpha_tag, tgl.tg_group, NULL::INTEGER, NULL::JSONB,
CASE WHEN tgl.tg_group IS NULL THEN NULL ELSE ARRAY[tgl.tg_group] END,
TRUE, NULL::JSONB, 1.0, TRUE learned, sys.id, sys.name
FROM talkgroups_learned tgl
JOIN systems sys ON tgl.system_id = sys.id
WHERE ignored IS NOT TRUE;

-- name: GetSystemName :one
SELECT name FROM systems WHERE id = @system_id;

-- name: UpdateTalkgroup :one
UPDATE talkgroups
SET
	name = COALESCE(sqlc.narg('name'), name),
	alpha_tag = COALESCE(sqlc.narg('alpha_tag'), alpha_tag),
	tg_group = COALESCE(sqlc.narg('tg_group'), tg_group),
	frequency = COALESCE(sqlc.narg('frequency'), frequency),
	metadata = COALESCE(sqlc.narg('metadata'), metadata),
	tags = COALESCE(sqlc.narg('tags'), tags),
	alert = COALESCE(sqlc.narg('alert'), alert),
	alert_config = COALESCE(sqlc.narg('alert_config'), alert_config),
	weight = COALESCE(sqlc.narg('weight'), weight)
WHERE id = sqlc.narg('id') OR (system_id = sqlc.narg('system_id') AND tgid = sqlc.narg('tgid'))
RETURNING *;

-- name: AddTalkgroupWithLearnedFlag :exec
INSERT INTO talkgroups (
	system_id,
	tgid,
	learned
) VALUES(
	@system_id,
	@tgid,
	't'
);

-- name: AddLearnedTalkgroup :one
INSERT INTO talkgroups_learned(
	system_id,
	tgid,
	name,
	alpha_tag,
	tg_group
) VALUES (
	@system_id,
	@tgid,
	sqlc.narg('name'),
	sqlc.narg('alpha_tag'),
	sqlc.narg('tg_group')
) RETURNING id;
