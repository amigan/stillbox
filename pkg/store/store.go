package store

import (
	"context"

	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
	"dynatron.me/x/stillbox/pkg/users"
)

type Store interface {
	TG() tgstore.Store
	User() users.Store
}

type store struct {
	tg   tgstore.Store
	user users.Store
}

func (s *store) TG() tgstore.Store {
	return s.tg
}

func (s *store) User() users.Store {
	return s.user
}

func New() Store {
	return &store{
		tg:   tgstore.NewCache(),
		user: users.NewStore(),
	}
}

type storeCtxKey string

const StoreCtxKey storeCtxKey = "store"

func CtxWithStore(ctx context.Context, s Store) context.Context {
	return context.WithValue(ctx, StoreCtxKey, s)
}

func FromCtx(ctx context.Context) Store {
	s, ok := ctx.Value(StoreCtxKey).(Store)
	if !ok {
		return New()
	}

	return s
}
