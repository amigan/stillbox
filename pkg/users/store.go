package users

import (
	"context"
	"errors"

	"dynatron.me/x/stillbox/internal/cache"
	"dynatron.me/x/stillbox/pkg/database"
)

var (
	ErrNoSuchUser = errors.New("no such user")
)

type Store interface {
	// GetUser gets a user by UID.
	GetUser(ctx context.Context, username string) (*User, error)

	// UserPrefs gets the preferences for the specified user and app name.
	UserPrefs(ctx context.Context, username string, appName string) ([]byte, error)

	// SetUserPrefs sets the preferences for the specified user and app name.
	SetUserPrefs(ctx context.Context, username string, appName string, prefs []byte) error

	// Invalidate clears the user cache.
	Invalidate()

	// UpdateUser updates a user's record
	UpdateUser(ctx context.Context, username string, user UserUpdate) error

	// GetUserByAPIKey gets a user by API key.
	GetAPIKey(ctx context.Context, key string) (database.GetAPIKeyRow, error)
}

type postgresStore struct {
	cache.Cache[string, *User]
	db database.Store
}

func NewStore(db database.Store) *postgresStore {
	return &postgresStore{
		Cache: cache.New[string, *User](),
		db:    db,
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
		panic("no users store in context")
	}

	return s
}

func (s *postgresStore) Invalidate() {
	s.Clear()
}

type UserUpdate struct {
	Email   *string `json:"email"`
	IsAdmin *bool   `json:"isAdmin"`
}

func (s *postgresStore) UpdateUser(ctx context.Context, username string, user UserUpdate) error {
	dbu, err := s.db.UpdateUser(ctx, username, user.Email, user.IsAdmin)
	if err != nil {
		return err
	}

	s.Set(username, fromDBUser(dbu))

	return nil
}

func (s *postgresStore) GetUser(ctx context.Context, username string) (*User, error) {
	u, has := s.Get(username)
	if has {
		return u, nil
	}

	dbu, err := s.db.GetUserByUsername(ctx, username)
	if err != nil {
		if database.IsNoRows(err) {
			return nil, ErrNoSuchUser
		}

		return nil, err
	}

	u = fromDBUser(dbu)
	s.Set(username, u)

	return u, nil
}

func (s *postgresStore) UserPrefs(ctx context.Context, username string, appName string) ([]byte, error) {
	u, err := s.GetUser(ctx, username)
	if err != nil {
		return nil, err
	}

	prefs, err := s.db.GetAppPrefs(ctx, appName, int(u.ID))
	if err != nil {
		return nil, err
	}

	return []byte(prefs), err
}

func (s *postgresStore) SetUserPrefs(ctx context.Context, username string, appName string, prefs []byte) error {
	u, err := s.GetUser(ctx, username)
	if err != nil {
		return err
	}

	return s.db.SetAppPrefs(ctx, appName, prefs, int(u.ID))
}

func (s *postgresStore) GetAPIKey(ctx context.Context, b64hash string) (database.GetAPIKeyRow, error) {
	return s.db.GetAPIKey(ctx, b64hash)
}
