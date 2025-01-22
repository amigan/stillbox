package shares

import (
	"context"
	"time"

	"dynatron.me/x/stillbox/pkg/rbac/entities"
	"github.com/rs/zerolog/log"
)

const (
	PruneInterval = time.Hour * 4
)

type Service interface {
	Shares

	Go(ctx context.Context)
}

type service struct {
	postgresStore
}

func (s *service) Go(ctx context.Context) {
	ctx = entities.CtxWithSubject(ctx, &entities.SystemServiceSubject{Name: "share"})

	tick := time.NewTicker(PruneInterval)

	for {
		select {
		case <-tick.C:
			err := s.Prune(ctx)
			if err != nil {
				log.Error().Err(err).Msg("share prune failed")
			}
		case <-ctx.Done():
			tick.Stop()
			return
		}
	}
}

func NewService() *service {
	return &service{}
}
