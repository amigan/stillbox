-- name: GetTalkgroupsWithAnyTags :many
SELECT * FROM talkgroups
WHERE tags @> ARRAY[$1];

-- name: GetTalkgroupsWithAllTags :many
SELECT * FROM talkgroups
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
SELECT * FROM talkgroups
WHERE id = systg2id(sqlc.arg(system_id), sqlc.arg(tgid));

-- name: GetTalkgroupsByPackedIDs :many
SELECT * FROM talkgroups tg
JOIN systems sys ON tg.system_id = sys.id
WHERE tg.id = ANY($1::INT8[]);

-- name: GetTalkgroupWithLearned :one
SELECT
tg.id, tg.system_id, sys.name system_name, tg.tgid, tg.name,
tg.tg_group, tg.frequency, tg.metadata, tg.tags, tg.alpha_tag,
tg.alert, tg.weight, tg.alert_config,
FALSE learned
FROM talkgroups tg
JOIN systems sys ON tg.system_id = sys.id
WHERE tg.id = systg2id(sqlc.arg(system_id), sqlc.arg(tgid))
UNION
SELECT
tgl.id::INT8, tgl.system_id::INT4, sys.name system_name, tgl.tgid::INT4, tgl.name,
tgl.alpha_tag, NULL::INTEGER, NULL::JSONB,
CASE WHEN tgl.alpha_tag IS NULL THEN NULL ELSE ARRAY[tgl.alpha_tag] END, tgl.alpha_tag,
TRUE, 1.0, NULL::JSONB,
TRUE learned
FROM talkgroups_learned tgl
JOIN systems sys ON tgl.system_id = sys.id
WHERE tgl.system_id = sqlc.arg(system_id) AND tgl.tgid = sqlc.arg(tgid) AND ignored IS NOT TRUE;

-- name: GetTalkgroupWithLearnedByPackedIDs :many
SELECT
tg.id, tg.system_id, sys.name system_name, tg.tgid, tg.name,
tg.tg_group, tg.frequency, tg.metadata, tg.tags, tg.alpha_tag,
tg.alert, tg.weight, tg.alert_config,
FALSE learned
FROM talkgroups tg
JOIN systems sys ON tg.system_id = sys.id
WHERE tg.id = ANY($1::INT8[])
UNION
SELECT
tgl.id::INT8, tgl.system_id::INT4, sys.name system_name, tgl.tgid::INT4, tgl.name,
tgl.alpha_tag, NULL::INTEGER, NULL::JSONB,
CASE WHEN tgl.alpha_tag IS NULL THEN NULL ELSE ARRAY[tgl.alpha_tag] END, tgl.alpha_tag,
TRUE, 1.0, NULL::JSONB,
TRUE learned
FROM talkgroups_learned tgl
JOIN systems sys ON tgl.system_id = sys.id
WHERE systg2id(tgl.system_id, tgl.tgid) = ANY($1::INT8[]) AND ignored IS NOT TRUE;

-- name: GetSystemName :one
SELECT name FROM systems WHERE id = sqlc.arg(system_id);
