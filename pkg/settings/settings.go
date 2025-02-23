package settings

import (
	"context"
	"encoding/json"
	"errors"

	"dynatron.me/x/stillbox/internal/cache"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/rbac"
	"dynatron.me/x/stillbox/pkg/rbac/entities"
	"dynatron.me/x/stillbox/pkg/services"
	"dynatron.me/x/stillbox/pkg/users"
)

var (
	ErrNoSetting = errors.New("no such setting")
)

type Store interface {
	// Get gets a setting and unmarshals it into dst.
	Get(ctx context.Context, name string) (Setting, error)

	// Set sets a setting.
	Set(ctx context.Context, name string, val Setting) error

	// PrimeDefaults primes the cache with defaults and sets them in the database if they do not exist.
	PrimeDefaults(ctx context.Context, def Defaults) error

	// Delete removes a setting.
	Delete(ctx context.Context, name string) error
}

type Setting interface {
}

type postgresStore struct {
	c cache.Cache[string, Setting]
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

func New() *postgresStore {
	s := &postgresStore{
		c: cache.New[string, Setting](),
	}

	return s
}

func (s *postgresStore) Get(ctx context.Context, name string) (Setting, error) {
	_, err := rbac.Check(ctx, rbac.UseResource(entities.ResourceSetting), rbac.WithActions(entities.ActionRead))
	if err != nil {
		return nil, err
	}

	ci, has := s.c.Get(name)
	if has {
		return ci, nil
	}
	db := database.FromCtx(ctx)

	cBytes, err := db.GetSetting(ctx, name)
	if err != nil {
		if database.IsNoRows(err) {
			return nil, ErrNoSetting
		}

		return nil, err
	}

	err = json.Unmarshal(cBytes, ci)
	if err != nil {
		return nil, err
	}

	s.c.Set(name, ci)

	return ci, nil
}

func (s *postgresStore) Set(ctx context.Context, name string, val Setting) error {
	subj, err := rbac.Check(ctx, rbac.UseResource(entities.ResourceSetting), rbac.WithActions(entities.ActionCreate, entities.ActionUpdate))
	if err != nil {
		return err
	}

	b, err := json.Marshal(val)
	if err != nil {
		return err
	}

	var uid *int32
	switch u := subj.(type) {
	case *entities.SystemServiceSubject:
		// uid remains null
	case *users.User:
		u, err := users.FromSubject(subj)
		if err != nil {
			return err
		}

		uid = u.ID.Int32Ptr()
	default:
		return rbac.ErrBadSubject
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
	_, err := rbac.Check(ctx, rbac.UseResource(entities.ResourceSetting), rbac.WithActions(entities.ActionDelete))
	if err != nil {
		return err
	}

	s.c.Delete(name)
	return database.FromCtx(ctx).DeleteSetting(ctx, name)
}

func (s *postgresStore) PrimeDefaults(ctx context.Context, def Defaults) error {
	for k, v := range def {
		_, err := s.Get(ctx, k)
		switch err {
		case nil:
		case ErrNoSetting:
			err = s.Set(ctx, k, v)
			if err != nil {
				return err
			}
		default:
			return err
		}
	}

	return nil
}
