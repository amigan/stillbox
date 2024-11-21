package alert

import (
	"context"
	"fmt"
	"time"

	"dynatron.me/x/stillbox/internal/trending"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/talkgroups"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"

	"github.com/jackc/pgx/v5/pgtype"
)

type Alert struct {
	ID         int
	Timestamp  time.Time
	TGName     string
	Talkgroup  *talkgroups.Talkgroup
	Score      trending.Score[talkgroups.ID]
	OrigScore  float64
	Weight     float32
	Suppressed bool
}

func (a *Alert) ToAddAlertParams() database.AddAlertParams {
	f32score := float32(a.Score.Score)
	f32origscore := float32(a.OrigScore)

	var origScore *float32
	if a.Score.Score != a.OrigScore {
		origScore = &f32origscore
	}

	return database.AddAlertParams{
		Time:      pgtype.Timestamptz{Time: a.Timestamp, Valid: true},
		SystemID:  int(a.Score.ID.System),
		TGID:      int(a.Score.ID.Talkgroup),
		Weight:    &a.Weight,
		Score:     &f32score,
		OrigScore: origScore,
		Notified:  !a.Suppressed,
	}
}

// Make creates an alert for later rendering or storage.
func Make(ctx context.Context, score trending.Score[talkgroups.ID], origScore float64) (Alert, error) {
	store := tgstore.From(ctx)
	d := Alert{
		Score:     score,
		Timestamp: time.Now(),
		Weight:    1.0,
		OrigScore: origScore,
	}

	tgRecord, err := store.TG(ctx, score.ID)
	switch err {
	case nil:
		d.Weight = tgRecord.Talkgroup.Weight
		d.TGName = tgRecord.String()
		d.Talkgroup = tgRecord
	default:
		system, has := store.SystemName(ctx, int(score.ID.System))
		if has {
			d.TGName = fmt.Sprintf("%s:%d", system, int(score.ID.Talkgroup))
		} else {
			d.TGName = fmt.Sprintf("%d:%d", int(score.ID.System), int(score.ID.Talkgroup))
		}
	}

	return d, nil
}
