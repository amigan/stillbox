package alert

import (
	"context"
	"fmt"
	"time"

	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/internal/trending"
	"dynatron.me/x/stillbox/pkg/calls/callstore"
	"dynatron.me/x/stillbox/pkg/talkgroups"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
)

type TxCtx struct {
	Date       time.Time
	Transcript string
}

type Alert struct {
	ID         int
	Timestamp  time.Time
	TGName     string
	Talkgroup  *talkgroups.Talkgroup
	Score      trending.Score[talkgroups.ID]
	OrigScore  float64
	Weight     float32
	Suppressed bool
	Context    []TxCtx
}

func (a *Alert) FillTranscriptContext(ctx context.Context, count uint, threshold, lookback jsontypes.Duration) error {
	cs := callstore.FromCtx(ctx)
	tc, err := cs.TranscriptContext(ctx, a.Score.ID, count, threshold, lookback)
	if err != nil {
		return err
	}

	for _, c := range tc {
		if c.Transcript == nil {
			continue
		}

		a.Context = append(a.Context, TxCtx{
			Date:       c.CallDate.Time,
			Transcript: *c.Transcript,
		})
	}

	return nil
}

// Make creates an alert for later rendering or storage.
func Make(ctx context.Context, score trending.Score[talkgroups.ID], origScore float64) (Alert, error) {
	store := tgstore.FromCtx(ctx)
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
