package users

import (
	"context"
	"errors"
	"strings"
	"time"

	"dynatron.me/x/stillbox/internal/cache"
	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/authz"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/services"
	"golang.org/x/crypto/bcrypt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
)

var (
	ErrNoSuchUser     = errors.New("no such user")
	ErrNoUIDSpecified = errors.New("no user ID specified")
	ErrDuplicateName  = errors.New("a key with that name already exists for that user")
	ErrBadPassword    = errors.New("bad password")
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

	// AddUser adds a user to the store.
	AddUser(ctx context.Context, user *User) (*User, error)

	// ValidatePassword does a time-constant validation of the password and returns the User.
	ValidatePassword(ctx context.Context, username, password string) (*User, error)

	// HUP invalidates the cache.
	HUP(*config.Config)
}

type store struct {
	cache.Cache[string, *User]
	db database.Store
}

func NewStore(db database.Store) *store {
	return &store{
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

func (s *store) Invalidate() {
	s.Clear()
}

func (s *store) HUP(_ *config.Config) {
	s.Invalidate()
}

type UserUpdate struct {
	Email    *string  `json:"email"`
	RealName *string  `json:"realName"`
	Roles    []string `json:"roles"`
}

func (s *store) AddUser(ctx context.Context, user *User) (*User, error) {
	_, err := authz.Check(ctx, user, authz.WithActions(entities.ActionCreate))
	if err != nil {
		return nil, err
	}

	// trim spaces
	user.Username = strings.TrimSpace(user.Username)

	// validate the record
	// user.Password in this context is *unhashed*. Normally this is not the case.
	if user.Username == "" {
		return nil, errors.New("invalid username")
	}

	hashPw, err := user.Password.Hash()
	if err != nil {
		return nil, err
	}

	dbu, err := s.db.CreateUser(ctx, database.CreateUserParams{
		Username: user.Username,
		Password: string(hashPw),
		Email:    user.Email,
		RealName: user.RealName,
		Roles:    user.Roles,
	})
	if err != nil {
		cv := database.ConstraintViolation(err)
		if cv != nil {
			return nil, cv
		}

		return nil, err
	}

	newUser := FromDBUser(dbu)

	s.Set(user.Username, newUser)

	return newUser.Mask(), nil
}

func (s *store) UpdateUser(ctx context.Context, username string, input UserUpdate) error {
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

func (s *store) GetUser(ctx context.Context, username string) (*User, error) {
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

func (s *store) GetUserPrivCheck(ctx context.Context, username string) (*User, error) {
	u, err := s.GetUser(ctx, username)
	if err != nil {
		return nil, err
	}

	return userPrivMask(ctx, u), nil
}

func (s *store) UserPrefs(ctx context.Context, username string, appName string) ([]byte, error) {
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

func (s *store) SetUserPrefs(ctx context.Context, username string, appName string, prefs []byte) error {
	u, err := s.GetUser(ctx, username)
	if err != nil {
		return err
	}

	return s.db.SetAppPrefs(ctx, appName, prefs, int(u.ID))
}

func (s *store) GetAPIKey(ctx context.Context, keyKind APIKeyKind, b64hash *string, jwtID *uuid.UUID) (database.GetAPIKeyRow, error) {
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

	if k.Disabled {
		return database.GetAPIKeyRow{}, ErrAPIKeyExpired
	}

	if k.Expires != nil && time.Now().After(*k.Expires) {
		return database.GetAPIKeyRow{}, ErrAPIKeyExpired
	}

	return k, nil
}

func (s *store) RecordLogin(ctx context.Context, username, source string) error {
	now := time.Now()
	ip, err := common.RemoteAddr(source)
	if err != nil {
		return err
	}

	return s.db.RecordUserLogin(ctx, username, &now, &ip)
}

func (s *store) CreateAPIKey(ctx context.Context, ak *APIKey) error {
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

func (s *store) ValidatePassword(ctx context.Context, username, password string) (*User, error) {
	user, err := s.GetUser(ctx, username)
	if err != nil || user == nil {
		log.Error().Str("username", username).Err(err).Msg("getUsers failed")
		_ = bcrypt.CompareHashAndPassword([]byte("thisPreventsTimingAttacks"), []byte(password))
		return user, ErrBadPassword
	}

	pwHash, err := user.Password.Hash()
	if err != nil {
		return nil, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(pwHash), []byte(password))
	if err != nil {
		return user, ErrBadPassword
	}

	return user, nil
}

func HashPassword(pass string) (string, error) {
	hashpw, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hashpw), err
}

func (s *store) ChangePassword(ctx context.Context, username string, newPassword string) (err error) {
	s.Cache.DeleteAndHoldLock(username, func() {
		var pw Password
		pw, err = NewPlainPassword(newPassword)
		if err != nil {
			return
		}

		var pwHash string
		pwHash, err = pw.Hash()
		if err != nil {
			return
		}

		err = s.db.UpdatePassword(ctx, username, pwHash)
	})

	return err
}
