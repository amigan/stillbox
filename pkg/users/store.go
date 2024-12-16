package users

import (
	"context"

	"dynatron.me/x/stillbox/pkg/database"
)

type Store interface {
	// UserPrefs gets the preferences for the specified user and app name.
	UserPrefs(ctx context.Context, uid int32, appName string) ([]byte, error)

	// SetUserPrefs sets the preferences for the specified user and app name.
	SetUserPrefs(ctx context.Context, uid int32, appName string, prefs []byte) error
}

type store struct {
}

func NewStore() *store {
	return new(store)
}

type storeCtxKey string

const StoreCtxKey storeCtxKey = "store"

func CtxWithStore(ctx context.Context, s Store) context.Context {
	return context.WithValue(ctx, StoreCtxKey, s)
}

func FromCtx(ctx context.Context) Store {
	s, ok := ctx.Value(StoreCtxKey).(Store)
	if !ok {
		return NewStore()
	}

	return s
}

func (s *store) UserPrefs(ctx context.Context, uid int32, appName string) ([]byte, error) {
	db := database.FromCtx(ctx)

	prefs, err := db.GetAppPrefs(ctx, appName, int(uid))
	if err != nil {
		return nil, err
	}

	return []byte(prefs), err
}

func (s *store) SetUserPrefs(ctx context.Context, uid int32, appName string, prefs []byte) error {
	db := database.FromCtx(ctx)

	return db.SetAppPrefs(ctx, appName, prefs, int(uid))
}
