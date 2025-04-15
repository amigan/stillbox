package settings

import (
	"context"
	"encoding/json"
	"errors"

	"dynatron.me/x/stillbox/internal/cache"
	"dynatron.me/x/stillbox/pkg/authz"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/services"
	"dynatron.me/x/stillbox/pkg/users"
)

var (
	ErrNoSetting = errors.New("no such setting")
)

type Store interface {
	// Get gets a setting and unmarshals it into dst.
	Get(ctx context.Context, name string) (Setting, error)

	// GetPrefs gets system prefs for an app name.
	GetPrefs(ctx context.Context, appName string) (json.RawMessage, error)

	// SetPrefs sets system prefs for an app name.
	SetPrefs(ctx context.Context, appName string, val Setting) error

	// Set sets a setting.
	Set(ctx context.Context, name string, val Setting) error

	// Delete removes a setting.
	Delete(ctx context.Context, name string) error
}

type Setting interface {
}

type postgresStore struct {
	c        cache.Cache[string, Setting]
	defaults Defaults
}

type storeCtxKey string

const StoreCtxKey storeCtxKey = "store"

func CtxWithStore(ctx context.Context, s Store) context.Context {
	return services.WithValue(ctx, StoreCtxKey, s)
}

func FromCtx(ctx context.Context) Store {
	s, ok := services.Value(ctx, StoreCtxKey).(Store)
	if !ok {
		panic("no settings store in context")
	}

	return s
}

func New(defaults Defaults) *postgresStore {
	s := &postgresStore{
		c:        cache.New[string, Setting](),
		defaults: defaults,
	}

	return s
}

func (s *postgresStore) defaultPrefs(appName string) (json.RawMessage, bool) {
	d, has := s.defaults[prefsName(appName)].(json.RawMessage)
	return d, has
}

func (s *postgresStore) getJSONB(ctx context.Context, name string) (json.RawMessage, error) {
	db := database.FromCtx(ctx)

	sRes, err := db.GetSetting(ctx, name)
	if err != nil && database.IsNoRows(err) {
		return nil, ErrNoSetting
	}

	return sRes, err
}

func prefsName(appName string) string {
	return "prefs." + appName
}

func (s *postgresStore) Get(ctx context.Context, name string) (Setting, error) {
	_, err := authz.Check(ctx, authz.UseResource(entities.ResourceSetting), authz.WithActions(entities.ActionRead))
	if err != nil {
		return nil, err
	}

	ci, has := s.c.Get(name)
	if has {
		return ci, nil
	}
	sRes, err := s.getJSONB(ctx, name)
	if err != nil && errors.Is(err, ErrNoSetting) {
		def, hasDefault := s.defaults[name]
		if hasDefault {
			return def, nil
		}

		return nil, err
	}

	err = json.Unmarshal(sRes, &ci)
	if err != nil {
		return nil, err
	}

	s.c.Set(name, ci)

	return ci, nil
}

func (s *postgresStore) GetPrefs(ctx context.Context, appName string) (json.RawMessage, error) {
	_, err := authz.Check(ctx, authz.UseResource(entities.ResourceSetting), authz.WithActions(entities.ActionRead))
	if err != nil {
		return nil, err
	}

	prName := prefsName(appName)
	ci, has := s.c.Get(prName)
	if has {
		rm, isRM := ci.(json.RawMessage)
		if isRM {
			return rm, nil
		}
	}

	p, err := s.getJSONB(ctx, prName)
	if errors.Is(err, ErrNoSetting) {
		def, hasDefault := s.defaultPrefs(prName)
		if hasDefault {
			return def, nil
		}

		return nil, err
	}

	s.c.Set(prName, p)

	return p, nil
}

func (s *postgresStore) SetPrefs(ctx context.Context, appName string, val Setting) error {
	return s.Set(ctx, prefsName(appName), val)
}

func (s *postgresStore) Set(ctx context.Context, name string, val Setting) error {
	subj, err := authz.Check(ctx, authz.UseResource(entities.ResourceSetting), authz.WithActions(entities.ActionCreate, entities.ActionUpdate))
	if err != nil {
		return err
	}

	b, err := json.Marshal(val)
	if err != nil {
		return err
	}

	var uid *int32
	switch subj.(type) {
	case *entities.SystemServiceSubject:
		// uid remains null
	case *users.User:
		u, err := users.FromSubject(subj)
		if err != nil {
			return err
		}

		uid = u.ID.Int32Ptr()
	default:
		return authz.ErrBadSubject
	}

	db := database.FromCtx(ctx)

	err = db.SetSetting(ctx, name, uid, b)
	if err != nil {
		return err
	}

	s.c.Set(name, val)

	return nil
}

func (s *postgresStore) Delete(ctx context.Context, name string) error {
	_, err := authz.Check(ctx, authz.UseResource(entities.ResourceSetting), authz.WithActions(entities.ActionDelete))
	if err != nil {
		return err
	}

	s.c.Delete(name)
	return database.FromCtx(ctx).DeleteSetting(ctx, name)
}
