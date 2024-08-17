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

-- name: GetTalkgroupWithLearned :one
SELECT
tg.id, tg.system_id, tg.tgid, tg.name,
tg.tg_group, tg.frequency, tg.metadata, tg.tags,
FALSE learned
FROM talkgroups tg
WHERE id = systg2id(sqlc.arg(system_id), sqlc.arg(tgid))
UNION
SELECT
tgl.id::INT8, tgl.system_id::INT4, tgl.tgid::INT4, tgl.name,
tgl.group_tag, NULL::INTEGER, NULL::JSONB,
CASE WHEN tgl.group_tag IS NULL THEN NULL ELSE ARRAY[tgl.group_tag] END,
TRUE learned
FROM talkgroups_learned tgl
WHERE system_id = sqlc.arg(system_id) AND tgid = sqlc.arg(tgid) AND ignored IS NOT TRUE;
