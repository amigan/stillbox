-- name: GetTalkgroupsWithAnyTags :many
SELECT system_id, talkgroup_id FROM talkgroups_tags
WHERE tags @> ARRAY[$1];

-- name: GetTalkgroupsWithAllTags :many
SELECT system_id, talkgroup_id FROM talkgroups_tags
WHERE tags && ARRAY[$1];

-- name: GetTalkgroupTags :one
SELECT tags FROM talkgroups_tags
WHERE talkgroup_id = $1;

-- name: SetTalkgroupTags :exec
INSERT INTO talkgroups_tags(system_id, talkgroup_id, tags) VALUES($1, $2, $3)
ON CONFLICT (system_id, talkgroup_id) DO UPDATE SET tags = $3;

