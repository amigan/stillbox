package users

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"

	"dynatron.me/x/stillbox/internal/cache"
	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/jsontypes"
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
	GetAPIKey(ctx context.Context, key string) (database.GetAPIKeyRow, error)

	// CreateAPIKey creates a new API key.
	CreateAPIKey(ctx context.Context, uid *UserID, name *string, expiresAt *time.Time, disabled bool) (*APIKey, error)

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
	Email *string  `json:"email"`
	Roles []string `json:"roles"`
}

func (s *postgresStore) UpdateUser(ctx context.Context, username string, user UserUpdate) error {
	dbu, err := s.db.UpdateUser(ctx, username, user.Email, user.Roles)
	if err != nil {
		return err
	}

	s.Set(username, FromDBUser(dbu))

	return nil
}

// userPrivMask masks privileged fields if the subject is not permitted to read them.
// It copies the user it is passed.
func userPrivMask(ctx context.Context, user *User) *User {
	_, err := authz.Check(ctx, user, authz.WithActions(entities.ActionReadPrivileged))
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

func (s *postgresStore) GetAPIKey(ctx context.Context, b64hash string) (database.GetAPIKeyRow, error) {
	return s.db.GetAPIKey(ctx, b64hash)
}

func (s *postgresStore) RecordLogin(ctx context.Context, username, source string) error {
	ts := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	ip, err := common.RemoteAddr(source)
	if err != nil {
		return err
	}

	return s.db.RecordUserLogin(ctx, username, ts, &ip)
}

func (s *postgresStore) CreateAPIKey(ctx context.Context, owner *UserID, name *string, expiresAt *time.Time, disabled bool) (*APIKey, error) {
	var userID UserID
	if owner != nil {
		userID = *owner
	} else {
		sub := entities.SubjectFrom(ctx)
		if u, isUser := sub.(*User); isUser {
			userID = u.ID
		} else {
			return nil, ErrNoUIDSpecified
		}
	}

	ak := &APIKey{
		Owner:     userID,
		Name:      name,
		CreatedAt: jsontypes.Time(time.Now()),
		Expires:   (*jsontypes.Time)(expiresAt),
		Disabled:  disabled,
	}

	_, err := authz.Check(ctx, ak, authz.WithActions(entities.ActionCreate))
	if err != nil {
		return nil, err
	}

	var expires pgtype.Timestamp

	if expiresAt != nil {
		expires = pgtype.Timestamp{
			Time:  expiresAt.UTC(),
			Valid: true,
		}
	}

	// generate it after auth
	uu := uuid.New()
	hash256 := sha256.Sum256([]byte(uu.String()))
	hashedKey := base64.StdEncoding.EncodeToString(hash256[:])

	err = s.db.CreateAPIKey(ctx, database.CreateAPIKeyParams{
		CreatedAt: ak.CreatedAt.Time(),
		Name:      ak.Name,
		Owner:     ak.Owner.Int(),
		Expires:   expires,
		Disabled:  disabled,
		HashedKey: hashedKey,
	})
	if err != nil {
		switch {
		case database.IsConstraintViolation(err, "api_keys_owner_name_key"):
			return nil, ErrDuplicateName
		default:
			return nil, err
		}
	}

	ju := jsontypes.UUID(uu)
	ak.Key = &ju

	return ak, nil
}
