package database

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const getTalkgroupWithLearnedByPackedIDsTest = `-- name: GetTalkgroupWithLearnedByPackedIDs :many
SELECT
tg.id, tg.system_id, tg.tgid, tg.name, tg.alpha_tag, tg.tg_group, tg.frequency, tg.metadata, tg.tags, tg.alert, tg.alert_config, tg.weight, sys.id, sys.name,
FALSE learned
FROM talkgroups tg
JOIN systems sys ON tg.system_id = sys.id
WHERE tg.id = ANY($1::INT8[])
UNION
SELECT
tgl.id::INT8, tgl.system_id::INT4, tgl.tgid::INT4, tgl.name,
tgl.alpha_tag, tgl.alpha_tag, NULL::INTEGER, NULL::JSONB,
CASE WHEN tgl.alpha_tag IS NULL THEN NULL ELSE ARRAY[tgl.alpha_tag] END,
TRUE, NULL::JSONB, 1.0, sys.id, sys.name,
TRUE learned
FROM talkgroups_learned tgl
JOIN systems sys ON tgl.system_id = sys.id
WHERE systg2id(tgl.system_id, tgl.tgid) = ANY($1::INT8[]) AND ignored IS NOT TRUE
`
const getTalkgroupWithLearnedTest = `-- name: GetTalkgroupWithLearned :one
SELECT
tg.id, tg.system_id, tg.tgid, tg.name, tg.alpha_tag, tg.tg_group, tg.frequency, tg.metadata, tg.tags, tg.alert, tg.alert_config, tg.weight, sys.id, sys.name,
FALSE learned
FROM talkgroups tg
JOIN systems sys ON tg.system_id = sys.id
WHERE tg.id = systg2id($1, $2)
UNION
SELECT
tgl.id::INT8, tgl.system_id::INT4, tgl.tgid::INT4, tgl.name,
tgl.alpha_tag, tgl.alpha_tag, NULL::INTEGER, NULL::JSONB,
CASE WHEN tgl.alpha_tag IS NULL THEN NULL ELSE ARRAY[tgl.alpha_tag] END,
TRUE, NULL::JSONB, 1.0, sys.id, sys.name,
TRUE learned
FROM talkgroups_learned tgl
JOIN systems sys ON tgl.system_id = sys.id
WHERE tgl.system_id = $1 AND tgl.tgid = $2 AND ignored IS NOT TRUE
`

func TestQueryColumnsMatch(t *testing.T) {
	require.Equal(t, getTalkgroupWithLearnedByPackedIDsTest, getTalkgroupWithLearnedByPackedIDs)
	require.Equal(t, getTalkgroupWithLearnedTest, getTalkgroupWithLearned)
}
