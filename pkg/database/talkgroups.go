package database

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

type talkgroupQuerier interface {
	GetTalkgroupsWithLearnedBySysTGID(ctx context.Context, ids TGTuples) ([]GetTalkgroupsRow, error)
	GetTalkgroupsBySysTGID(ctx context.Context, ids TGTuples) ([]GetTalkgroupsRow, error)
	BulkSetTalkgroupTags(ctx context.Context, tgs TGTuples, tags []string) error
}

type TGTuples [2][]uint32

const (
	TGConstraintName  = "calls_system_talkgroup_fkey"
	SysConstraintName = "talkgroups_system_id_fkey"
)

func IsTGConstraintViolation(e error) bool {
	return IsConstraintViolation(e, TGConstraintName)
}

func IsSystemConstraintViolation(e error) bool {
	return IsConstraintViolation(e, SysConstraintName)
}

func IsConstraintViolation(e error, constraintName string) bool {
	var err *pgconn.PgError
	if errors.As(e, &err) && err.Code == "23503" && err.ConstraintName == constraintName {
		return true
	}

	return false
}

func MakeTGTuples(cap int) TGTuples {
	return [2][]uint32{
		make([]uint32, 0, cap),
		make([]uint32, 0, cap),
	}
}

func (t *TGTuples) Append(sys, tg uint32) {
	t[0] = append(t[0], sys)
	t[1] = append(t[1], tg)
}

// Below queries are here because sqlc refuses to parse unnest(x, y)

const getTalkgroupsWithLearnedBySysTGID = `SELECT
tg.id, tg.system_id, tg.tgid, tg.name, tg.alpha_tag, tg.tg_group, tg.frequency, tg.metadata, tg.tags, tg.alert, tg.alert_config, tg.weight, sys.id, sys.name, tg.learned, tg.ignored
FROM talkgroups tg
JOIN systems sys ON tg.system_id = sys.id
JOIN UNNEST($1::INT4[], $2::INT4[]) AS tgt(sys, tg) ON (tg.system_id = tgt.sys AND tg.tgid = tgt.tg);`

type GetTalkgroupsRow struct {
	Talkgroup Talkgroup `json:"talkgroup"`
	System    System    `json:"system"`
}

func (q *Queries) GetTalkgroupsWithLearnedBySysTGID(ctx context.Context, ids TGTuples) ([]GetTalkgroupsRow, error) {
	rows, err := q.db.Query(ctx, getTalkgroupsWithLearnedBySysTGID, ids[0], ids[1])
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
			&i.Talkgroup.AlertConfig,
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

const getTalkgroupsBySysTGID = `SELECT tg.id, tg.system_id, tg.tgid, tg.name, tg.alpha_tag, tg.tg_group, tg.frequency, tg.metadata, tg.tags, tg.alert, tg.alert_config, tg.weight, tg.learned, tg.ignored, sys.id, sys.name FROM talkgroups tg
JOIN systems sys ON tg.system_id = sys.id
JOIN UNNEST($1::INT4[], $2::INT4[]) AS tgt(sys, tg) ON (tg.system_id = tgt.sys AND tg.tgid = tgt.tg)
WHERE tg.learned IS NOT TRUE;`

func (q *Queries) GetTalkgroupsBySysTGID(ctx context.Context, ids TGTuples) ([]GetTalkgroupsRow, error) {
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
			&i.Talkgroup.AlertConfig,
			&i.Talkgroup.Weight,
			&i.Talkgroup.Learned,
			&i.Talkgroup.Ignored,
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

const bulkSetTalkgroupTags = `UPDATE talkgroups tg SET tags = $3 FROM UNNEST($1::INT4[], $2::INT4[]) AS tgt(sys, tg) WHERE (tg.system_id = tgt.sys AND tg.tgid = tgt.tg);`

func (q *Queries) BulkSetTalkgroupTags(ctx context.Context, tgs TGTuples, tags []string) error {
	_, err := q.db.Exec(ctx, bulkSetTalkgroupTags, tgs[0], tgs[1], tags)
	return err
}
