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

-- name: GetAllTalkgroupTags :many
SELECT UNNEST(tgs.tags)::TEXT tag FROM talkgroups tgs GROUP BY tag ORDER BY COUNT(*) DESC;

-- name: GetTalkgroupWithLearned :one
SELECT
sqlc.embed(tg), sqlc.embed(sys)
FROM talkgroups tg
JOIN systems sys ON tg.system_id = sys.id
WHERE (tg.system_id, tg.tgid) = (@system_id, @tgid);

-- name: GetTalkgroupsWithLearnedBySystemP :many
SELECT
sqlc.embed(tg), sqlc.embed(sys)
FROM talkgroups tg
JOIN systems sys ON tg.system_id = sys.id
WHERE tg.system_id = @system AND
(CASE WHEN sqlc.narg('filter')::TEXT IS NOT NULL THEN (
		tg.tg_group ILIKE '%' || @filter || '%' OR
		tg.name ILIKE '%' || @filter || '%' OR
		tg.alpha_tag ILIKE '%' || @filter || '%' OR
		tg.tags @> ARRAY[LOWER(@filter)]
	) ELSE TRUE END)
ORDER BY
CASE WHEN @order_by::TEXT = 'tgid_asc' THEN (tg.system_id, tg.tgid) END ASC,
CASE WHEN @order_by = 'tgid_desc' THEN (tg.system_id, tg.tgid) END DESC,
CASE WHEN @order_by = 'group_asc' THEN tg.tg_group END ASC,
CASE WHEN @order_by = 'group_desc' THEN tg.tg_group END DESC,
CASE WHEN @order_by = 'id_asc' THEN tg.id END ASC,
CASE WHEN @order_by = 'id_desc' THEN tg.id END DESC,
CASE WHEN @order_by = 'name_asc' THEN tg.name END ASC,
CASE WHEN @order_by = 'name_desc' THEN tg.name END DESC,
CASE WHEN @order_by = 'alpha_asc' THEN tg.alpha_tag END ASC,
CASE WHEN @order_by = 'alpha_desc' THEN tg.alpha_tag END DESC
OFFSET sqlc.arg('offset') ROWS
FETCH NEXT sqlc.arg('per_page') ROWS ONLY
;

-- name: GetTalkgroupsWithLearnedBySystemCount :one
SELECT COUNT(*) FROM talkgroups tg
WHERE tg.system_id = @system AND
(CASE WHEN sqlc.narg('filter')::TEXT IS NOT NULL THEN (
		tg.tg_group ILIKE '%' || @filter || '%' OR
		tg.name ILIKE '%' || @filter || '%' OR
		tg.alpha_tag ILIKE '%' || @filter || '%' OR
		tg.tags @> ARRAY[LOWER(@filter)]
	) ELSE TRUE END)
;

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

-- name: GetTalkgroupsWithLearnedP :many
SELECT
sqlc.embed(tg), sqlc.embed(sys)
FROM talkgroups tg
JOIN systems sys ON tg.system_id = sys.id
WHERE ignored IS NOT TRUE AND
(CASE WHEN sqlc.narg('filter')::TEXT IS NOT NULL THEN (
		tg.tg_group ILIKE '%' || @filter || '%' OR
		tg.name ILIKE '%' || @filter || '%' OR
		tg.alpha_tag ILIKE '%' || @filter || '%' OR
		tg.tags @> ARRAY[LOWER(@filter)]
	) ELSE TRUE END)
ORDER BY
CASE WHEN @order_by::TEXT = 'tgid_asc' THEN (tg.system_id, tg.tgid) END ASC,
CASE WHEN @order_by = 'tgid_desc' THEN (tg.system_id, tg.tgid) END DESC,
CASE WHEN @order_by = 'group_asc' THEN tg.tg_group END ASC,
CASE WHEN @order_by = 'group_desc' THEN tg.tg_group END DESC,
CASE WHEN @order_by = 'id_asc' THEN tg.id END ASC,
CASE WHEN @order_by = 'id_desc' THEN tg.id END DESC,
CASE WHEN @order_by = 'name_asc' THEN tg.name END ASC,
CASE WHEN @order_by = 'name_desc' THEN tg.name END DESC,
CASE WHEN @order_by = 'alpha_asc' THEN tg.alpha_tag END ASC,
CASE WHEN @order_by = 'alpha_desc' THEN tg.alpha_tag END DESC
OFFSET sqlc.arg('offset') ROWS
FETCH NEXT sqlc.arg('per_page') ROWS ONLY
;

-- name: GetTalkgroupsWithLearnedCount :one
SELECT COUNT(*) FROM talkgroups tg
WHERE ignored IS NOT TRUE AND
(CASE WHEN sqlc.narg('filter')::TEXT IS NOT NULL THEN (
		tg.tg_group ILIKE '%' || @filter || '%' OR
		tg.name ILIKE '%' || @filter || '%' OR
		tg.alpha_tag ILIKE '%' || @filter || '%' OR
		tg.tags @> ARRAY[LOWER(@filter)]
	) ELSE TRUE END)
;

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
	alert_rules = COALESCE(sqlc.narg('alert_rules'), alert_rules),
	weight = COALESCE(sqlc.narg('weight'), weight),
	learned = COALESCE(sqlc.narg('learned'), learned)
WHERE id = sqlc.narg('id') OR (system_id = sqlc.narg('system_id') AND tgid = sqlc.narg('tgid'))
RETURNING *;

-- name: UpsertTalkgroup :batchone
INSERT INTO talkgroups AS tg (
	system_id, tgid, name, alpha_tag, tg_group, frequency, metadata, tags, alert, alert_rules, weight, learned
) VALUES (
	@system_id,
	@tgid,
	sqlc.narg('name'),
	sqlc.narg('alpha_tag'),
	sqlc.narg('tg_group'),
	sqlc.narg('frequency'),
	sqlc.narg('metadata'),
	COALESCE(sqlc.narg('tags'), '{}'::text[])::text[],
	COALESCE(sqlc.narg('alert'), TRUE),
	sqlc.narg('alert_rules'),
	COALESCE(sqlc.narg('weight'), 1.0)::numeric,
	COALESCE(sqlc.narg('learned'), FALSE)::boolean
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
	alert_rules = COALESCE(sqlc.narg('alert_rules'), tg.alert_rules),
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
	alert_rules,
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
	tg.alert_rules,
	tg.weight,
	tg.learned
FROM talkgroups tg WHERE tg.system_id = @system_id AND tg.tgid = @tgid;

-- name: AddLearnedTalkgroup :one
INSERT INTO talkgroups(
	system_id,
	tgid,
	learned,
	name,
	alpha_tag,
	tg_group
) VALUES (
	@system_id,
	@tgid,
	TRUE,
	sqlc.narg('name'),
	sqlc.narg('alpha_tag'),
	sqlc.narg('tg_group')
) RETURNING *;

-- name: RestoreTalkgroupVersion :one
INSERT INTO talkgroups(
	system_id,
	tgid,
	name,
	alpha_tag,
	tg_group,
	frequency,
	metadata,
	tags,
	alert,
	alert_rules,
	weight,
	learned,
	ignored
)
SELECT
	system_id,
	tgid,
	name,
	alpha_tag,
	tg_group,
	frequency,
	metadata,
	tags,
	alert,
	alert_rules,
	weight,
	learned,
	ignored
FROM talkgroup_versions tgv ON CONFLICT (system_id, tgid) DO UPDATE SET
	name = excluded.name,
	alpha_tag = excluded.alpha_tag,
	tg_group = excluded.tg_group,
	metadata = excluded.metadata,
	tags = excluded.tags,
	alert = excluded.alert,
	alert_rules = excluded.alert_rules,
	weight = excluded.weight,
	learned = excluded.learner,
	ignored = excluded.ignored
WHERE tgv.id = ANY(@version_ids)
RETURNING *;

-- name: DeleteTalkgroup :exec
DELETE FROM talkgroups WHERE system_id = @system_id AND tgid = @tg_id;

-- name: StoreDeletedTGVersion :exec
INSERT INTO talkgroup_versions(
	system_id,
	tgid,
	time,
	created_by,
	deleted
) VALUES(@system_id, @tg_id, NOW(), @submitter, TRUE);

-- name: CreateSystem :exec
INSERT INTO systems(id, name) VALUES(@id, @name);

-- name: DeleteSystem :exec
DELETE FROM systems WHERE id = @id;

-- name: GetIncidentTalkgroups :many
SELECT DISTINCT
	c.system,
	c.talkgroup
FROM incidents_calls ic 
JOIN calls c ON (c.id = ic.call_id AND c.call_date = ic.call_date) 
WHERE ic.incident_id = @incident_id;
