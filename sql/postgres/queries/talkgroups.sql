-- name: GetTalkgroupsWithAnyTags :many
SELECT * FROM talkgroups
WHERE tags @> ARRAY[$1];

-- name: GetTalkgroupsWithAllTags :many
SELECT * FROM talkgroups
WHERE tags && ARRAY[$1];

-- name: GetTalkgroupTags :one
SELECT tags FROM talkgroups
WHERE system_id = $1 AND tgid = $2;

-- name: SetTalkgroupTags :exec
UPDATE talkgroups SET tags = $1
WHERE system_id = $1 AND tgid = $2;
