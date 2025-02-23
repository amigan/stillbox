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
	Get(ctx context.Context, name string, dest interface{}) error

	// Set sets a setting.
	Set(ctx context.Context, name string, val interface{}) error

	// Delete removes a setting.
	Delete(ctx context.Context, name string) error
}

type postgresStore struct {
	c cache.Cache[string, []byte]
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
		c: cache.New[string, []byte](),
	}

	return s
}

func (s *postgresStore) Get(ctx context.Context, name string, dest interface{}) error {
	_, err := rbac.Check(ctx, rbac.UseResource(entities.ResourceSetting), rbac.WithActions(entities.ActionRead))
	if err != nil {
		return err
	}

	ci, has := s.c.Get(name)
	if !has {
		db := database.FromCtx(ctx)

		ci, err = db.GetSetting(ctx, name)
		if err != nil {
			if database.IsNoRows(err) {
				return ErrNoSetting
			}

			return err
		}

		s.c.Set(name, ci)
	}

	return json.Unmarshal(ci, dest)
}

func (s *postgresStore) Set(ctx context.Context, name string, val interface{}) error {
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

	s.c.Set(name, b)

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
