-- name: GetTalkgroupsWithAnyTags :many
SELECT sqlc.embed(talkgroups) FROM talkgroups
WHERE tags @> ARRAY[$1];

-- name: GetTalkgroupsWithAllTags :many
SELECT sqlc.embed(talkgroups) FROM talkgroups
WHERE tags && ARRAY[$1];

-- name: GetTalkgroupIDsByTags :many
SELECT system_id, tgid FROM talkgroups
WHERE (tags @> ARRAY[sqlc.arg(anyTags)])
AND (tags && ARRAY[sqlc.arg(allTags)])
AND NOT (tags @> ARRAY[sqlc.arg(notTags)]);

-- name: GetTalkgroupTags :one
SELECT tags FROM talkgroups
WHERE id = systg2id($1, $2);

-- name: SetTalkgroupTags :exec
UPDATE talkgroups SET tags = $3
WHERE id = systg2id($1, $2);

-- name: BulkSetTalkgroupTags :exec
UPDATE talkgroups SET tags = $2
WHERE id = ANY($1);

-- name: GetTalkgroup :one
SELECT sqlc.embed(talkgroups) FROM talkgroups
WHERE id = systg2id(sqlc.arg(system_id), sqlc.arg(tgid));

-- name: GetTalkgroupsByPackedIDs :many
SELECT sqlc.embed(tg), sqlc.embed(sys) FROM talkgroups tg
JOIN systems sys ON tg.system_id = sys.id
WHERE tg.id = ANY($1::INT8[]);

-- name: GetTalkgroupWithLearned :one
SELECT
sqlc.embed(tg), sqlc.embed(sys),
FALSE learned
FROM talkgroups tg
JOIN systems sys ON tg.system_id = sys.id
WHERE tg.id = systg2id(sqlc.arg(system_id), sqlc.arg(tgid))
UNION
SELECT
tgl.id::INT8, tgl.system_id::INT4, tgl.tgid::INT4, tgl.name,
tgl.alpha_tag, tgl.alpha_tag, NULL::INTEGER, NULL::JSONB,
CASE WHEN tgl.alpha_tag IS NULL THEN NULL ELSE ARRAY[tgl.alpha_tag] END,
TRUE, NULL::JSONB, 1.0, sys.id, sys.name,
TRUE learned
FROM talkgroups_learned tgl
JOIN systems sys ON tgl.system_id = sys.id
WHERE tgl.system_id = sqlc.arg(system_id) AND tgl.tgid = sqlc.arg(tgid) AND ignored IS NOT TRUE;

-- name: GetTalkgroupsWithLearnedByPackedIDs :many
SELECT
sqlc.embed(tg), sqlc.embed(sys),
FALSE learned
FROM talkgroups tg
JOIN systems sys ON tg.system_id = sys.id
WHERE tg.id = ANY($1::INT8[])
UNION
SELECT
tgl.id::INT8, tgl.system_id::INT4, tgl.tgid::INT4, tgl.name,
tgl.alpha_tag, tgl.alpha_tag, NULL::INTEGER, NULL::JSONB,
CASE WHEN tgl.alpha_tag IS NULL THEN NULL ELSE ARRAY[tgl.alpha_tag] END,
TRUE, NULL::JSONB, 1.0, sys.id, sys.name,
TRUE learned
FROM talkgroups_learned tgl
JOIN systems sys ON tgl.system_id = sys.id
WHERE systg2id(tgl.system_id, tgl.tgid) = ANY($1::INT8[]) AND ignored IS NOT TRUE;

-- name: GetTalkgroupsWithLearnedBySystem :many
SELECT
sqlc.embed(tg), sqlc.embed(sys),
FALSE learned
FROM talkgroups tg
JOIN systems sys ON tg.system_id = sys.id
WHERE tg.system_id = @system
UNION
SELECT
tgl.id::INT8, tgl.system_id::INT4, tgl.tgid::INT4, tgl.name,
tgl.alpha_tag, tgl.alpha_tag, NULL::INTEGER, NULL::JSONB,
CASE WHEN tgl.alpha_tag IS NULL THEN NULL ELSE ARRAY[tgl.alpha_tag] END,
TRUE, NULL::JSONB, 1.0, sys.id, sys.name,
TRUE learned
FROM talkgroups_learned tgl
JOIN systems sys ON tgl.system_id = sys.id
WHERE tgl.system_id = @system AND ignored IS NOT TRUE;

-- name: GetTalkgroupsWithLearned :many
SELECT
sqlc.embed(tg), sqlc.embed(sys),
FALSE learned
FROM talkgroups tg
JOIN systems sys ON tg.system_id = sys.id
UNION
SELECT
tgl.id::INT8, tgl.system_id::INT4, tgl.tgid::INT4, tgl.name,
tgl.alpha_tag, tgl.alpha_tag, NULL::INTEGER, NULL::JSONB,
CASE WHEN tgl.alpha_tag IS NULL THEN NULL ELSE ARRAY[tgl.alpha_tag] END,
TRUE, NULL::JSONB, 1.0, sys.id, sys.name,
TRUE learned
FROM talkgroups_learned tgl
JOIN systems sys ON tgl.system_id = sys.id
WHERE ignored IS NOT TRUE;

-- name: GetSystemName :one
SELECT name FROM systems WHERE id = sqlc.arg(system_id);

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
WHERE id = @id
RETURNING *;
