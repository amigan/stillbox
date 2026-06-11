package database

import (
	"context"
)

type talkgroupQuerier interface {
	GetTalkgroupsBySysTGID(ctx context.Context, ids TGTuplesU) ([]GetTalkgroupsRow, error)
	BulkSetTalkgroupTags(ctx context.Context, tgs TGTuplesU, tags []string) error
}

type TGTuplesU [2][]uint32
type TGTuples [2][]int32

type TGID struct {
	System    uint32 `db:"system"`
	Talkgroup uint32 `db:"tg"`
}

func IsTGConstraintViolation(e error) bool {
	return IsConstraintViolation(e, TGConstraintName)
}

func IsSystemConstraintViolation(e error) bool {
	return IsConstraintViolation(e, SysConstraintName)
}

func MakeTGTuples(cap int) TGTuplesU {
	return [2][]uint32{
		make([]uint32, 0, cap),
		make([]uint32, 0, cap),
	}
}

func (t *TGTuplesU) Append(sys, tg uint32) {
	t[0] = append(t[0], sys)
	t[1] = append(t[1], tg)
}

// Below queries are here because sqlc refuses to parse unnest(x, y)

const getTalkgroupsBySysTGID = `SELECT
tg.id, tg.system_id, tg.tgid, tg.name, tg.alpha_tag, tg.tg_group, tg.frequency, tg.metadata, tg.tags, tg.alert, tg.alert_rules, tg.weight, sys.id, sys.name, tg.learned, tg.ignored
FROM talkgroups tg
JOIN systems sys ON tg.system_id = sys.id
JOIN UNNEST($1::INT4[], $2::INT4[]) AS tgt(sys, tg) ON (tg.system_id = tgt.sys AND tg.tgid = tgt.tg);`

func (q *Queries) GetTalkgroupsBySysTGID(ctx context.Context, ids TGTuplesU) ([]GetTalkgroupsRow, error) {
	rows, err := q.db.Query(ctx, getTalkgroupsBySysTGID, ids[0], ids[1])
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
			&i.Talkgroup.TGID,
			&i.Talkgroup.Name,
			&i.Talkgroup.AlphaTag,
			&i.Talkgroup.TGGroup,
			&i.Talkgroup.Frequency,
			&i.Talkgroup.Metadata,
			&i.Talkgroup.Tags,
			&i.Talkgroup.Alert,
			&i.Talkgroup.AlertRules,
			&i.Talkgroup.Weight,
			&i.System.ID,
			&i.System.Name,
			&i.Talkgroup.Learned,
			&i.Talkgroup.Ignored,
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

const bulkSetTalkgroupTags = `UPDATE talkgroups tg SET tags = $3 FROM UNNEST($1::INT4[], $2::INT4[]) AS tgt(sys, tg) WHERE (tg.system_id = tgt.sys AND tg.tgid = tgt.tg);`

func (q *Queries) BulkSetTalkgroupTags(ctx context.Context, tgs TGTuplesU, tags []string) error {
	_, err := q.db.Exec(ctx, bulkSetTalkgroupTags, tgs[0], tgs[1], tags)
	return err
}
