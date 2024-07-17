-- name: GetTalkgroupsWithAnyTags :many
SELECT * FROM talkgroups_tags
WHERE tags @> ARRAY[$1];

-- name: GetTalkgroupsWithAllTags :many
SELECT * FROM talkgroups_tags
WHERE tags && ARRAY[$1];

-- name: GetTalkgroupTags :one
SELECT tags FROM talkgroups_tags
WHERE talkgroup_id = $1;

-- name: SetTalkgroupTags :exec
INSERT INTO talkgroups_tags(talkgroup_id, tags) VALUES($1, $2)
ON CONFLICT (talkgroup_id) DO UPDATE SET tags = $2;

