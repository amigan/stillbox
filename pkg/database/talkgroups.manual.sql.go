package database

import (
	"context"
	"database/sql/driver"
)

type TalkgroupT struct {
	System    uint32 `json:"sys"`
	Talkgroup uint32 `json:"tg"`
}

func (t TalkgroupT) Value() (driver.Value, error) {
	return [2]uint32{t.System, t.Talkgroup}, nil
}

const getTalkgroupsWithLearnedByPackedIDs = `-- name: GetTalkgroupsWithLearnedByPackedIDs :many
SELECT
tg.id, tg.system_id, tg.tgid, tg.name, tg.alpha_tag, tg.tg_group, tg.frequency, tg.metadata, tg.tags, tg.alert, tg.alert_config, tg.weight, sys.id, sys.name,
FALSE learned
FROM talkgroups tg
JOIN systems sys ON tg.system_id = sys.id
WHERE (tg.system_id, tg.tgid) = ANY($1)
UNION
SELECT
NULL::UUID, tgl.system_id::INT4, tgl.tgid::INT4, tgl.name,
tgl.alpha_tag, tgl.alpha_tag, NULL::INTEGER, NULL::JSONB,
CASE WHEN tgl.alpha_tag IS NULL THEN NULL ELSE ARRAY[tgl.alpha_tag] END,
TRUE, NULL::JSONB, 1.0, sys.id, sys.name,
TRUE learned
FROM talkgroups_learned tgl
JOIN systems sys ON tgl.system_id = sys.id
WHERE (tgl.system_id, tgl.tgid) = ANY($1);
`

type GetTalkgroupsWithLearnedByPackedIDsRow struct {
	Talkgroup Talkgroup `json:"talkgroup"`
	System    System    `json:"system"`
	Learned   bool      `json:"learned"`
}

func (q *Queries) GetTalkgroupsWithLearnedByPackedIDs(ctx context.Context, ids []TalkgroupT) ([]GetTalkgroupsWithLearnedByPackedIDsRow, error) {
	rows, err := q.db.Query(ctx, getTalkgroupsWithLearnedByPackedIDs, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetTalkgroupsWithLearnedByPackedIDsRow
	for rows.Next() {
		var i GetTalkgroupsWithLearnedByPackedIDsRow
		if err := rows.Scan(
			&i.Talkgroup.ID,
			&i.Talkgroup.SystemID,
			&i.Talkgroup.Tgid,
			&i.Talkgroup.Name,
			&i.Talkgroup.AlphaTag,
			&i.Talkgroup.TgGroup,
			&i.Talkgroup.Frequency,
			&i.Talkgroup.Metadata,
			&i.Talkgroup.Tags,
			&i.Talkgroup.Alert,
			&i.Talkgroup.AlertConfig,
			&i.Talkgroup.Weight,
			&i.System.ID,
			&i.System.Name,
			&i.Learned,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getTalkgroupsByPackedIDs = `-- name: GetTalkgroupsByPackedIDs :many
SELECT tg.id, tg.system_id, tg.tgid, tg.name, tg.alpha_tag, tg.tg_group, tg.frequency, tg.metadata, tg.tags, tg.alert, tg.alert_config, tg.weight, sys.id, sys.name FROM talkgroups tg
JOIN systems sys ON tg.system_id = sys.id
WHERE (tg.system_id, tg.tgid) = ANY($1)
`

type GetTalkgroupsByPackedIDsRow struct {
	Talkgroup Talkgroup `json:"talkgroup"`
	System    System    `json:"system"`
}

func (q *Queries) GetTalkgroupsByPackedIDs(ctx context.Context, idtuple []TalkgroupT) ([]GetTalkgroupsByPackedIDsRow, error) {
	rows, err := q.db.Query(ctx, getTalkgroupsByPackedIDs, idtuple)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetTalkgroupsByPackedIDsRow
	for rows.Next() {
		var i GetTalkgroupsByPackedIDsRow
		if err := rows.Scan(
			&i.Talkgroup.ID,
			&i.Talkgroup.SystemID,
			&i.Talkgroup.Tgid,
			&i.Talkgroup.Name,
			&i.Talkgroup.AlphaTag,
			&i.Talkgroup.TgGroup,
			&i.Talkgroup.Frequency,
			&i.Talkgroup.Metadata,
			&i.Talkgroup.Tags,
			&i.Talkgroup.Alert,
			&i.Talkgroup.AlertConfig,
			&i.Talkgroup.Weight,
			&i.System.ID,
			&i.System.Name,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}


