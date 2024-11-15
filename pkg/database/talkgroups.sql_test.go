package database

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const getTalkgroupWithLearnedTest = `-- name: GetTalkgroupWithLearned :one
SELECT
tg.id, tg.system_id, tg.tgid, tg.name, tg.alpha_tag, tg.tg_group, tg.frequency, tg.metadata, tg.tags, tg.alert, tg.alert_config, tg.weight, sys.id, sys.name,
FALSE learned
FROM talkgroups tg
JOIN systems sys ON tg.system_id = sys.id
WHERE (tg.system_id, tg.tgid) = ($1, $2)
UNION
SELECT
NULL::UUID, tgl.system_id::INT4, tgl.tgid::INT4, tgl.name,
tgl.alpha_tag, tgl.alpha_tag, NULL::INTEGER, NULL::JSONB,
CASE WHEN tgl.alpha_tag IS NULL THEN NULL ELSE ARRAY[tgl.alpha_tag] END,
TRUE, NULL::JSONB, 1.0, sys.id, sys.name,
TRUE learned
FROM talkgroups_learned tgl
JOIN systems sys ON tgl.system_id = sys.id
WHERE tgl.system_id = $1 AND tgl.tgid = $2 AND ignored IS NOT TRUE
`

const getTalkgroupsWithLearnedBySystemTest = `-- name: GetTalkgroupsWithLearnedBySystem :many
SELECT
tg.id, tg.system_id, tg.tgid, tg.name, tg.alpha_tag, tg.tg_group, tg.frequency, tg.metadata, tg.tags, tg.alert, tg.alert_config, tg.weight, sys.id, sys.name,
FALSE learned
FROM talkgroups tg
JOIN systems sys ON tg.system_id = sys.id
WHERE tg.system_id = $1
UNION
SELECT
NULL::UUID, tgl.system_id::INT4, tgl.tgid::INT4, tgl.name,
tgl.alpha_tag, tgl.alpha_tag, NULL::INTEGER, NULL::JSONB,
CASE WHEN tgl.alpha_tag IS NULL THEN NULL ELSE ARRAY[tgl.alpha_tag] END,
TRUE, NULL::JSONB, 1.0, sys.id, sys.name,
TRUE learned
FROM talkgroups_learned tgl
JOIN systems sys ON tgl.system_id = sys.id
WHERE tgl.system_id = $1 AND ignored IS NOT TRUE
`

const getTalkgroupsWithLearnedTest = `-- name: GetTalkgroupsWithLearned :many
SELECT
tg.id, tg.system_id, tg.tgid, tg.name, tg.alpha_tag, tg.tg_group, tg.frequency, tg.metadata, tg.tags, tg.alert, tg.alert_config, tg.weight, sys.id, sys.name,
FALSE learned
FROM talkgroups tg
JOIN systems sys ON tg.system_id = sys.id
UNION
SELECT
NULL::UUID, tgl.system_id::INT4, tgl.tgid::INT4, tgl.name,
tgl.alpha_tag, tgl.alpha_tag, NULL::INTEGER, NULL::JSONB,
CASE WHEN tgl.alpha_tag IS NULL THEN NULL ELSE ARRAY[tgl.alpha_tag] END,
TRUE, NULL::JSONB, 1.0, sys.id, sys.name,
TRUE learned
FROM talkgroups_learned tgl
JOIN systems sys ON tgl.system_id = sys.id
WHERE ignored IS NOT TRUE
`

func TestQueryColumnsMatch(t *testing.T) {
	require.Equal(t, getTalkgroupWithLearnedTest, getTalkgroupWithLearned)
	require.Equal(t, getTalkgroupsWithLearnedBySystemTest, getTalkgroupsWithLearnedBySystem)
	require.Equal(t, getTalkgroupsWithLearnedTest, getTalkgroupsWithLearned)
}
