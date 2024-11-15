package database

import (
	"context"
	"database/sql/driver"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

type TalkgroupT struct {
	System    uint32 `json:"system_id"`
	Talkgroup uint32 `json:"tgid"`
}

type TalkgroupTs []TalkgroupT

func (t TalkgroupTs) Nest() (sys []uint32, tg []uint32) {
	sys = make([]uint32, len(t))
	tg = make([]uint32, len(t))

	for i := range t {
		sys[i] = t[i].System
		tg[i] = t[i].Talkgroup
	}

	return
}

func (t TalkgroupT) Value() (driver.Value, error) {
	return [2]uint32{t.System, t.Talkgroup}, nil
}

func (t TalkgroupT) TextValue() (pgtype.Text, error) {
	return pgtype.Text{String: fmt.Sprintf("%d:%d", t.System, t.Talkgroup)}, nil
}

const getTalkgroupsWithLearnedByPackedIDs = `-- name: GetTalkgroupsWithLearnedByPackedIDs :many
SELECT
tg.id, tg.system_id, tg.tgid, tg.name, tg.alpha_tag, tg.tg_group, tg.frequency, tg.metadata, tg.tags, tg.alert, tg.alert_config, tg.weight, sys.id, sys.name,
FALSE learned
FROM talkgroups tg
JOIN systems sys ON tg.system_id = sys.id
JOIN UNNEST($1::INT4[], $2::INT4[]) AS tgt(sys, tg) ON (tg.system_id = tgt.sys AND tg.tgid = tgt.tg)
UNION
SELECT
NULL::UUID, tgl.system_id::INT4, tgl.tgid::INT4, tgl.name,
tgl.alpha_tag, tgl.alpha_tag, NULL::INTEGER, NULL::JSONB,
CASE WHEN tgl.alpha_tag IS NULL THEN NULL ELSE ARRAY[tgl.alpha_tag] END,
TRUE, NULL::JSONB, 1.0, sys.id, sys.name,
TRUE learned
FROM talkgroups_learned tgl
JOIN systems sys ON tgl.system_id = sys.id
JOIN UNNEST($1::INT4[], $2::INT4[]) AS tgt(sys, tg) ON (tgl.system_id = tgt.sys AND tgl.tgid = tgt.tg);`

type GetTalkgroupsRow struct {
	Talkgroup Talkgroup `json:"talkgroup"`
	System    System    `json:"system"`
	Learned   bool      `json:"learned"`
}

func (q *Queries) GetTalkgroupsWithLearnedByPackedIDs(ctx context.Context, ids TalkgroupTs) ([]GetTalkgroupsRow, error) {
	sysAr, tgAr := ids.Nest()
	rows, err := q.db.Query(ctx, getTalkgroupsWithLearnedByPackedIDs, sysAr, tgAr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetTalkgroupsRow
	for rows.Next() {
		var i GetTalkgroupsRow
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
JOIN UNNEST($1::INT4[], $2::INT4[]) AS tgt(sys, tg) ON (tg.system_id = tgt.sys AND tg.tgid = tgt.tg)
`

func (q *Queries) GetTalkgroupsByPackedIDs(ctx context.Context, ids TalkgroupTs) ([]GetTalkgroupsRow, error) {
	sysAr, tgAr := ids.Nest()
	rows, err := q.db.Query(ctx, getTalkgroupsByPackedIDs, sysAr, tgAr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetTalkgroupsRow
	for rows.Next() {
		var i GetTalkgroupsRow
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
