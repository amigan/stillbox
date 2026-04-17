package alertstore

import (
	"context"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/alerting/alert"
	"dynatron.me/x/stillbox/pkg/authz"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/services"
	"dynatron.me/x/stillbox/pkg/talkgroups"
)

type Store interface {
	// AddAlert adds an alert to the database.
	AddAlert(ctx context.Context, a *alert.Alert) error

	// GetAlert gets an alert from the database based on the provided URL tag.
	GetAlert(ctx context.Context, urlTag string) (*alert.Base, error)
}

type store struct {
	db database.Store
}

func New(db database.Store) Store {
	return &store{db: db}
}

type storeCtxKey string

const StoreCtxKey storeCtxKey = "store"

func CtxWithStore(ctx context.Context, s Store) context.Context {
	return services.WithValue(ctx, StoreCtxKey, s)
}

func FromCtx(ctx context.Context) Store {
	s, ok := services.Value(ctx, StoreCtxKey).(Store)
	if !ok {
		panic("no alert store in context")
	}

	return s
}

func (s *store) GetAlert(ctx context.Context, urlTag string) (*alert.Base, error) {
	dba, err := s.db.GetAlertByURLTag(ctx, &urlTag)
	if err != nil {
		return nil, err
	}

	a := dbToBaseAlert(dba)

	_, err = authz.Check(ctx, a, authz.WithActions(entities.ActionCreate))
	if err != nil {
		return nil, err
	}

	return a, nil
}

func (s *store) AddAlert(ctx context.Context, a *alert.Alert) error {
	_, err := authz.Check(ctx, a, authz.WithActions(entities.ActionCreate))
	if err != nil {
		return err
	}

	ap := toAddAlertParams(a)

	return s.db.AddAlert(ctx, ap)
}

func dbToBaseAlert(d database.Alert) *alert.Base {
	weight := float32(1.0)
	if d.Weight != nil {
		weight = *d.Weight
	}

	score := 0.0
	if d.Score != nil {
		score = float64(*d.Score)
	}

	origScore := score
	if d.OrigScore != nil {
		origScore = float64(*d.OrigScore)
	}

	a := &alert.Base{
		ID:         d.ID,
		Timestamp:  d.Time,
		TGID:       talkgroups.TG(d.SystemID, d.TGID),
		OrigScore:  origScore,
		Weight:     weight,
		Suppressed: !d.Notified,
		URLTag:     d.URLTag,
	}

	if d.ContextWindow.Valid {
		a.ContextWindow = &common.TimeRange{
			Begin: d.ContextWindow.Lower.Time,
			End:   d.ContextWindow.Upper.Time,
		}
	}

	return a
}

func toAddAlertParams(a *alert.Alert) database.AddAlertParams {
	f32score := float32(a.Score.Score)
	f32origscore := float32(a.OrigScore)

	var origScore *float32
	if a.Score.Score != a.OrigScore {
		origScore = &f32origscore
	}

	a.ComputeContextRange()

	return database.AddAlertParams{
		Time:          a.Timestamp,
		SystemID:      int(a.Score.ID.System),
		TGID:          int(a.Score.ID.Talkgroup),
		Weight:        &a.Weight,
		Score:         &f32score,
		OrigScore:     origScore,
		Notified:      !a.Suppressed,
		ContextWindow: a.ContextWindow.TSTZRange(),
		URLTag:        a.URLTag,
	}
}
