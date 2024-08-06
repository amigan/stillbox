-- name: GetTalkgroupsWithAnyTags :many
SELECT * FROM talkgroups
WHERE tags @> ARRAY[$1];

-- name: GetTalkgroupsWithAllTags :many
SELECT * FROM talkgroups
WHERE tags && ARRAY[$1];

-- name: GetTalkgroupTags :one
SELECT tags FROM talkgroups
WHERE id = systg2id($1, $2);

-- name: SetTalkgroupTags :exec
UPDATE talkgroups SET tags = $3
WHERE id = systg2id($1, $2);

-- name: BulkSetTalkgroupTags :exec
UPDATE talkgroups SET tags = $2
WHERE id = ANY($1);
