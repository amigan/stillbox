package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const getTalkgroupWithLearnedTest = `-- name: GetTalkgroupWithLearned :one
SELECT
tg.id, tg.system_id, tg.tgid, tg.name, tg.alpha_tag, tg.tg_group, tg.frequency, tg.metadata, tg.tags, tg.alert, tg.alert_config, tg.weight, tg.learned, sys.id, sys.name
FROM talkgroups tg
JOIN systems sys ON tg.system_id = sys.id
WHERE (tg.system_id, tg.tgid) = ($1, $2) AND tg.learned IS NOT TRUE
UNION
SELECT
tgl.id, tgl.system_id::INT4, tgl.tgid::INT4, tgl.name,
tgl.alpha_tag, tgl.tg_group, NULL::INTEGER, NULL::JSONB,
CASE WHEN tgl.tg_group IS NULL THEN NULL ELSE ARRAY[tgl.tg_group] END,
NOT tgl.ignored, NULL::JSONB, 1.0, TRUE learned, sys.id, sys.name
FROM talkgroups_learned tgl
JOIN systems sys ON tgl.system_id = sys.id
WHERE tgl.system_id = $1 AND tgl.tgid = $2 AND ignored IS NOT TRUE
`

const getTalkgroupsWithLearnedBySystemTest = `-- name: GetTalkgroupsWithLearnedBySystem :many
SELECT
tg.id, tg.system_id, tg.tgid, tg.name, tg.alpha_tag, tg.tg_group, tg.frequency, tg.metadata, tg.tags, tg.alert, tg.alert_config, tg.weight, tg.learned, sys.id, sys.name
FROM talkgroups tg
JOIN systems sys ON tg.system_id = sys.id
WHERE tg.system_id = $1 AND tg.learned IS NOT TRUE
UNION
SELECT
tgl.id, tgl.system_id::INT4, tgl.tgid::INT4, tgl.name,
tgl.alpha_tag, tgl.tg_group, NULL::INTEGER, NULL::JSONB,
CASE WHEN tgl.tg_group IS NULL THEN NULL ELSE ARRAY[tgl.tg_group] END,
NOT tgl.ignored, NULL::JSONB, 1.0, TRUE learned, sys.id, sys.name
FROM talkgroups_learned tgl
JOIN systems sys ON tgl.system_id = sys.id
WHERE tgl.system_id = $1 AND ignored IS NOT TRUE
`

const getTalkgroupsWithLearnedTest = `-- name: GetTalkgroupsWithLearned :many
SELECT
tg.id, tg.system_id, tg.tgid, tg.name, tg.alpha_tag, tg.tg_group, tg.frequency, tg.metadata, tg.tags, tg.alert, tg.alert_config, tg.weight, tg.learned, sys.id, sys.name
FROM talkgroups tg
JOIN systems sys ON tg.system_id = sys.id
WHERE tg.learned IS NOT TRUE
UNION
SELECT
tgl.id, tgl.system_id::INT4, tgl.tgid::INT4, tgl.name,
tgl.alpha_tag, tgl.tg_group, NULL::INTEGER, NULL::JSONB,
CASE WHEN tgl.tg_group IS NULL THEN NULL ELSE ARRAY[tgl.tg_group] END,
NOT tgl.ignored, NULL::JSONB, 1.0, TRUE learned, sys.id, sys.name
FROM talkgroups_learned tgl
JOIN systems sys ON tgl.system_id = sys.id
WHERE ignored IS NOT TRUE
`

func TestQueryColumnsMatch(t *testing.T) {
	assert.Equal(t, getTalkgroupWithLearnedTest, getTalkgroupWithLearned)
	assert.Equal(t, getTalkgroupsWithLearnedBySystemTest, getTalkgroupsWithLearnedBySystem)
	assert.Equal(t, getTalkgroupsWithLearnedTest, getTalkgroupsWithLearned)
}
