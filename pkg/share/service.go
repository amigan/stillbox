package share

import (
	"context"
	"time"

	"dynatron.me/x/stillbox/pkg/rbac"
	"github.com/rs/zerolog/log"
)

const (
	PruneInterval = time.Hour * 4
)

type Service interface {
	ShareStore() Store

	Go(ctx context.Context)
}

type service struct {
	Store
}

func (s *service) ShareStore() Store {
	return s.Store
}

func (s *service) Go(ctx context.Context) {
	ctx = rbac.CtxWithSubject(ctx, &rbac.SystemServiceSubject{Name: "share"})

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
	return &service{
		Store: NewStore(),
	}
}
