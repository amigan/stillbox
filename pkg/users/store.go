package users

import (
	"context"
	"errors"
	"time"

	"dynatron.me/x/stillbox/internal/cache"
	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/authz"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/services"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrNoSuchUser     = errors.New("no such user")
	ErrNoUIDSpecified = errors.New("no user ID specified")
	ErrDuplicateName  = errors.New("a key with that name already exists for that user")
)

type Store interface {
	// GetUser gets a user by username. This is for use by the system, not for presentation to the API.
	GetUser(ctx context.Context, username string) (*User, error)

	// GetUserPrivCheck gets a user by  username, checking context Subject and masking.
	GetUserPrivCheck(ctx context.Context, username string) (*User, error)

	// UserPrefs gets the preferences for the specified user and app name.
	UserPrefs(ctx context.Context, username string, appName string) ([]byte, error)

	// SetUserPrefs sets the preferences for the specified user and app name.
	SetUserPrefs(ctx context.Context, username string, appName string, prefs []byte) error

	// Invalidate clears the user cache.
	Invalidate()

	// UpdateUser updates a user's record
	UpdateUser(ctx context.Context, username string, user UserUpdate) error

	// RecordLogin records a users's login.
	RecordLogin(ctx context.Context, username, source string) error

	// GetUserByAPIKey gets a user by API key.
	GetAPIKey(ctx context.Context, keyKind APIKeyKind, b64hash *string, jwtID *uuid.UUID) (database.GetAPIKeyRow, error)

	// CreateAPIKey creates a new API key.
	CreateAPIKey(ctx context.Context, ak *APIKey) error

	// ChangePassword changes a user's password.
	ChangePassword(ctx context.Context, username string, newPassword string) error

	// HUP invalidates the cache.
	HUP(*config.Config)
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
	return services.WithValue(ctx, StoreCtxKey, s)
}

func FromCtx(ctx context.Context) Store {
	s, ok := services.Value(ctx, StoreCtxKey).(Store)
	if !ok {
		panic("no users store in context")
	}

	return s
}

func (s *postgresStore) Invalidate() {
	s.Clear()
}

func (s *postgresStore) HUP(_ *config.Config) {
	s.Invalidate()
}

type UserUpdate struct {
	Email    *string  `json:"email"`
	RealName *string  `json:"realName"`
	Roles    []string `json:"roles"`
}

func (s *postgresStore) UpdateUser(ctx context.Context, username string, input UserUpdate) error {
	dbu, err := s.db.GetUserByUsername(ctx, username)
	if err != nil {
		return err
	}

	_, err = authz.Check(ctx, FromDBUser(dbu), authz.WithActions(entities.ActionUpdate))
	if err != nil {
		return err
	}

	dbu, err = s.db.UpdateUser(ctx, database.UpdateUserParams{
		Username: username,
		Email:    input.Email,
		RealName: input.RealName,
		Roles:    input.Roles,
	})
	if err != nil {
		return err
	}

	s.Set(username, FromDBUser(dbu))

	return nil
}

// userPrivMask masks privileged fields if the subject is not permitted to read them.
// It copies the user it is passed.
func userPrivMask(ctx context.Context, user *User) *User {
	_, err := authz.Check(ctx, user, authz.WithActions(entities.ActionReadPrivileged), authz.WithTry())
	if err != nil { // mask unprivileged
		return user.Mask()
	}

	return user
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

	u = FromDBUser(dbu)
	s.Set(username, u)

	return u, nil
}

func (s *postgresStore) GetUserPrivCheck(ctx context.Context, username string) (*User, error) {
	u, err := s.GetUser(ctx, username)
	if err != nil {
		return nil, err
	}

	return userPrivMask(ctx, u), nil
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

func (s *postgresStore) GetAPIKey(ctx context.Context, keyKind APIKeyKind, b64hash *string, jwtID *uuid.UUID) (database.GetAPIKeyRow, error) {
	if !keyKind.Valid() {
		return database.GetAPIKeyRow{}, ErrAPIKeyKindInvalid
	}

	var pgjwt pgtype.UUID
	if jwtID != nil {
		pgjwt = pgtype.UUID{
			Bytes: *jwtID,
			Valid: true,
		}
	}
	k, err := s.db.GetAPIKey(ctx, b64hash, pgjwt, int(keyKind))
	if err != nil {
		return database.GetAPIKeyRow{}, err
	}

	if k.Expires != nil && time.Now().After(*k.Expires) {
		return database.GetAPIKeyRow{}, ErrAPIKeyExpired
	}

	return k, nil
}

func (s *postgresStore) RecordLogin(ctx context.Context, username, source string) error {
	now := time.Now()
	ip, err := common.RemoteAddr(source)
	if err != nil {
		return err
	}

	return s.db.RecordUserLogin(ctx, username, &now, &ip)
}

func (s *postgresStore) CreateAPIKey(ctx context.Context, ak *APIKey) error {
	var jwtid pgtype.UUID
	if ak.JWTID != nil {
		jwtid = pgtype.UUID{
			Bytes: *ak.JWTID,
			Valid: true,
		}
	}

	err := s.db.CreateAPIKey(ctx, database.CreateAPIKeyParams{
		CreatedAt: ak.CreatedAt.Time(),
		Name:      ak.Name,
		Kind:      int(ak.Kind),
		OwnerID:   ak.OwnerID.Int(),
		Expires:   (*time.Time)(ak.Expires),
		Disabled:  ak.Disabled,
		HashedKey: ak.Hash,
		JwtID:     jwtid,
		Scopes:    ak.Scopes,
	})
	if err != nil {
		switch {
		case database.IsConstraintViolation(err, database.APIKeysOwnerIDNameKey):
			return ErrDuplicateName
		default:
			return err
		}
	}

	return nil
}

func (s *postgresStore) ChangePassword(ctx context.Context, username string, newPassword string) (err error) {
	s.Cache.DeleteAndHoldLock(username, func() {
		err = s.db.UpdatePassword(ctx, username, newPassword)
	})

	return err
}
