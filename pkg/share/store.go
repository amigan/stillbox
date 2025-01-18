package share

import (
	"context"

	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/database"
)

type Store interface {
	// Get retreives a share record.
	Get(ctx context.Context, id string) (*Share, error)

	// Create stores a new share record.
	Create(ctx context.Context, share *Share) error

	// Delete deletes a share record.
	Delete(ctx context.Context, id string) error

	// Prune removes expired share records.
	Prune(ctx context.Context) error
}

type postgresStore struct {
}

func recToShare(share database.Share) *Share {
	return &Share{
		ID:         share.ID,
		Type:       EntityType(share.EntityType),
		EntityID:   share.EntityID,
		Expiration: jsontypes.TimePtrFromTSTZ(share.Expiration),
	}
}

func (s *postgresStore) Get(ctx context.Context, id string) (*Share, error) {
	db := database.FromCtx(ctx)
	rec, err := db.GetShare(ctx, id)
	if err != nil {
		return nil, err
	}

	return recToShare(rec), nil
}

func (s *postgresStore) Create(ctx context.Context, share *Share) error {
	db := database.FromCtx(ctx)
	err := db.CreateShare(ctx, database.CreateShareParams{
		ID:         share.ID,
		EntityType: string(share.Type),
		EntityID:   share.EntityID,
		Expiration: share.Expiration.PGTypeTSTZ(),
	})

	return err
}

func (s *postgresStore) Delete(ctx context.Context, id string) error {
	return database.FromCtx(ctx).DeleteShare(ctx, id)
}

func (s *postgresStore) Prune(ctx context.Context) error {
	return database.FromCtx(ctx).PruneShares(ctx)
}

func NewStore() *postgresStore {
	return new(postgresStore)
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
