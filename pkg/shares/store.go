package shares

import (
	"context"
	"errors"

	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/rbac"
	"dynatron.me/x/stillbox/pkg/users"
	"github.com/jackc/pgx/v5"
)

type Shares interface {
	// NewShare creates a new share.
	NewShare(ctx context.Context, sh CreateShareParams) (*Share, error)

	// Share retreives a share record.
	GetShare(ctx context.Context, id string) (*Share, error)

	// Create stores a new share record.
	Create(ctx context.Context, share *Share) error

	// Delete deletes a share record.
	Delete(ctx context.Context, id string) error

	// Prune removes expired share records.
	Prune(ctx context.Context) error
}

type postgresStore struct {
}

var (
	ErrNoShare = errors.New("no such share")
)

func recToShare(share database.Share) *Share {
	return &Share{
		ID:         share.ID,
		Type:       EntityType(share.EntityType),
		EntityID:   share.EntityID,
		Date:       jsontypes.TimePtrFromTSTZ(share.EntityDate),
		Expiration: jsontypes.TimePtrFromTSTZ(share.Expiration),
		Owner:      users.UserID(share.Owner),
	}
}

func (s *postgresStore) GetShare(ctx context.Context, id string) (*Share, error) {
	db := database.FromCtx(ctx)
	rec, err := db.GetShare(ctx, id)
	switch err {
	case nil:
		return recToShare(rec), nil
	case pgx.ErrNoRows:
		return nil, ErrNoShare
	default:
		return nil, err
	}
}

func (s *postgresStore) Create(ctx context.Context, share *Share) error {
	sub, err := users.UserCheck(ctx, new(Share), "create")
	if err != nil {
		return err
	}

	db := database.FromCtx(ctx)
	err = db.CreateShare(ctx, database.CreateShareParams{
		ID:         share.ID,
		EntityType: string(share.Type),
		EntityID:   share.EntityID,
		EntityDate: share.Date.PGTypeTSTZ(),
		Expiration: share.Expiration.PGTypeTSTZ(),
		Owner:      sub.ID.Int(),
	})

	return err
}

func (s *postgresStore) Delete(ctx context.Context, id string) error {
	_, err := rbac.Check(ctx, new(Share), rbac.WithActions(rbac.ActionDelete))
	if err != nil {
		return err
	}

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

func CtxWithStore(ctx context.Context, s Shares) context.Context {
	return context.WithValue(ctx, StoreCtxKey, s)
}

func FromCtx(ctx context.Context) Shares {
	s, ok := ctx.Value(StoreCtxKey).(Shares)
	if !ok {
		panic("no shares store in context")
	}

	return s
}
