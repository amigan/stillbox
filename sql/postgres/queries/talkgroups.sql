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
WHERE (tg.system_id, tg.tgid) = (@system_id, @tgid);

-- name: GetTalkgroupsWithLearnedBySystem :many
SELECT
sqlc.embed(tg), sqlc.embed(sys)
FROM talkgroups tg
JOIN systems sys ON tg.system_id = sys.id
WHERE tg.system_id = @system;

-- name: GetTalkgroupsWithLearned :many
SELECT
sqlc.embed(tg), sqlc.embed(sys)
FROM talkgroups tg
JOIN systems sys ON tg.system_id = sys.id
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
	weight = COALESCE(sqlc.narg('weight'), weight),
	learned = COALESCE(sqlc.narg('learned'), learned)
WHERE id = sqlc.narg('id') OR (system_id = sqlc.narg('system_id') AND tgid = sqlc.narg('tgid'))
RETURNING *;

-- name: UpsertTalkgroup :batchone
INSERT INTO talkgroups AS tg (
	system_id, tgid, name, alpha_tag, tg_group, frequency, metadata, tags, alert, alert_config, weight, learned
) VALUES (
	@system_id,
	@tgid,
	sqlc.narg('name'),
	sqlc.narg('alpha_tag'),
	sqlc.narg('tg_group'),
	sqlc.narg('frequency'),
	sqlc.narg('metadata'),
	sqlc.narg('tags'),
	sqlc.narg('alert'),
	sqlc.narg('alert_config'),
	sqlc.narg('weight'),
	sqlc.narg('learned')
)
ON CONFLICT (system_id, tgid) DO UPDATE
SET
	name = COALESCE(sqlc.narg('name'), tg.name),
	alpha_tag = COALESCE(sqlc.narg('alpha_tag'), tg.alpha_tag),
	tg_group = COALESCE(sqlc.narg('tg_group'), tg.tg_group),
	frequency = COALESCE(sqlc.narg('frequency'), tg.frequency),
	metadata = COALESCE(sqlc.narg('metadata'), tg.metadata),
	tags = COALESCE(sqlc.narg('tags'), tg.tags),
	alert = COALESCE(sqlc.narg('alert'), tg.alert),
	alert_config = COALESCE(sqlc.narg('alert_config'), tg.alert_config),
	weight = COALESCE(sqlc.narg('weight'), tg.weight),
	learned = COALESCE(sqlc.narg('learned'), tg.learned)
RETURNING *;

-- name: StoreTGVersion :batchexec
INSERT INTO talkgroup_versions(time, created_by,
	system_id,
	tgid,
	name,
	alpha_tag,
	tg_group,
	frequency,
	metadata,
	tags,
	alert,
	alert_config,
	weight,
	learned
) SELECT NOW(), @submitter,
	tg.system_id,
	tg.tgid,
	tg.name,
	tg.alpha_tag,
	tg.tg_group,
	tg.frequency,
	tg.metadata,
	tg.tags,
	tg.alert,
	tg.alert_config,
	tg.weight,
	tg.learned
FROM talkgroups tg WHERE tg.system_id = @system_id AND tg.tgid = @tgid;

-- name: AddTalkgroupWithLearnedFlag :exec
INSERT INTO talkgroups (
	system_id,
	tgid,
	learned
) VALUES(
	@system_id,
	@tgid,
	TRUE
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
