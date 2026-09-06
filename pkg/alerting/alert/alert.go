package alert

import (
	"context"
	"fmt"
	"slices"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/internal/trending"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/calls/callstore"
	"dynatron.me/x/stillbox/pkg/talkgroups"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
)

type TxCtx struct {
	Date       time.Time `json:"ts"`
	Transcript string    `json:"tx"`
}

type Base struct {
	ID            int               `json:"-"`
	Timestamp     time.Time         `json:"timestamp"`
	TGID          talkgroups.ID     `json:"tg"`
	OrigScore     float64           `json:"origScore,omitzero"`
	Weight        float32           `json:"weight"`
	Suppressed    bool              `json:"suppressed,omitzero"`
	ContextWindow *common.TimeRange `json:"contextWindow,omitempty"`
	URLTag        *string           `json:"urlTag,omitempty"`
}

func (a *Alert) ComputeContextRange() {
	if len(a.Context) > 0 {
		// Contexts should be sorted in timestamp descending order from FillTranscriptContext,
		// but this should be cheap.
		slices.SortFunc(a.Context, func(a, b TxCtx) int {
			if a.Date.Before(b.Date) {
				return -1
			}

			return 1
		})

		a.ContextWindow = &common.TimeRange{
			Begin: a.Context[0].Date,
			End:   a.Context[len(a.Context)-1].Date,
		}
	}
}

// Alert is a fat Alert record for rendering using notify templates.
type Alert struct {
	Base
	Talkgroup *talkgroups.Talkgroup
	TGName    string
	Score     trending.Score[talkgroups.ID]
	Context   []TxCtx
}

func (a *Base) GetResourceName() string {
	return entities.ResourceAlert
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
			Date:       c.CallDate,
			Transcript: *c.Transcript,
		})
	}

	return nil
}

type RenderedAlertBatch struct {
	Alerts   []RenderedAlert
	TopIdx   int
	TopScore float64
}

func (r *RenderedAlertBatch) Top() *RenderedAlert {
	if r.TopIdx > len(r.Alerts)-1 {
		return nil
	}

	return &r.Alerts[r.TopIdx]
}

type RenderedAlert struct {
	*Alert

	Subject string
	Body    string
	URL     string
}

const URLTagLength = 8

// Make creates an alert for later rendering or storage.
func Make(ctx context.Context, score trending.Score[talkgroups.ID], origScore float64) (Alert, error) {
	store := tgstore.FromCtx(ctx)

	urlTag := common.NanoID(URLTagLength)

	d := Alert{
		Base: Base{
			Timestamp: time.Now(),
			Weight:    1.0,
			OrigScore: origScore,
			TGID:      score.ID,
			URLTag:    &urlTag,
		},
		Score: score,
	}

	tgRecord, err := store.TG(ctx, score.ID)
	switch err {
	case nil:
		d.Weight = tgRecord.Talkgroup.Weight
		d.TGName = tgRecord.String()
		d.Talkgroup = tgRecord
	default:
		system, has, err := store.SystemName(ctx, int(score.ID.System))
		if err != nil {
			return Alert{}, err
		}
		if has {
			d.TGName = fmt.Sprintf("%s:%d", system, int(score.ID.Talkgroup))
		} else {
			d.TGName = fmt.Sprintf("%d:%d", int(score.ID.System), int(score.ID.Talkgroup))
		}
	}

	return d, nil
}
