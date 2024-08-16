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
SELECT * FROM talkgroups WHERE id = systg2id(sqlc.arg(system_id), sqlc.arg(tgid));
