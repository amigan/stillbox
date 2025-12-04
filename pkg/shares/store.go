package shares

import (
	"context"
	"errors"
	"fmt"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/authz"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/services"
	"dynatron.me/x/stillbox/pkg/users"
	"github.com/jackc/pgx/v5"
)

type SharesParams struct {
	common.Pagination
	Direction *common.SortDirection `json:"dir"`
}

type Shares interface {
	// NewShare creates a new share.
	NewShare(ctx context.Context, sh CreateShareParams) (*Share, error)

	// Share retrieves a share record.
	GetShare(ctx context.Context, id string) (*Share, error)

	// Shares retrieves shares visible by the context Subject.
	Shares(ctx context.Context, p SharesParams) (shares []*Share, totalCount int, err error)

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
		Date:       (*jsontypes.Time)(share.EntityDate),
		Expiration: (*jsontypes.Time)(share.Expiration),
		OwnerID:    users.UserID(share.OwnerID),
	}
}

func (s *postgresStore) GetShare(ctx context.Context, id string) (*Share, error) {
	_, err := authz.Check(ctx, authz.UseResource(entities.ResourceShare), authz.WithActions(entities.ActionRead))
	if err != nil {
		return nil, err
	}

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
		EntityDate: (*time.Time)(share.Date),
		Expiration: (*time.Time)(share.Expiration),
		OwnerID:    sub.ID.Int(),
	})

	return err
}

func (s *postgresStore) Delete(ctx context.Context, id string) error {
	sh, err := s.GetShare(ctx, id)
	if err != nil {
		return err
	}

	_, err = authz.Check(ctx, sh, authz.WithActions(entities.ActionDelete))
	if err != nil {
		return err
	}

	return database.FromCtx(ctx).DeleteShare(ctx, id)
}

func (s *postgresStore) Shares(ctx context.Context, p SharesParams) (shares []*Share, totalCount int, err error) {
	sub := entities.SubjectFrom(ctx)

	// ersatz RBAC: non-admin can only see their own shares
	var owner *int32
	switch s := sub.(type) {
	case *users.User:
		if !s.HasRole(entities.RoleAdmin) {
			owner = s.ID.Int32Ptr()
		} else {
			owner = nil
		}
	case *entities.SystemServiceSubject:
		owner = nil
	default:
		return nil, 0, authz.ErrAccessDenied
	}

	db := database.FromCtx(ctx)

	count, err := db.GetSharesPCount(ctx, owner)
	if err != nil {
		return nil, 0, fmt.Errorf("shares count: %w", err)
	}

	offset, perPage := p.Pagination.OffsetPerPage(100)
	dbParam := database.GetSharesPParams{
		OwnerID:   owner,
		Direction: p.Direction.DirString(common.DirAsc),
		Offset:    offset,
		PerPage:   perPage,
	}

	shs, err := db.GetSharesP(ctx, dbParam)
	if err != nil {
		return nil, 0, err
	}

	shares = make([]*Share, 0, len(shs))
	for _, v := range shs {
		s := recToShare(v.Share)
		s.Owner = &v.Username
		shares = append(shares, s)
	}

	return shares, int(count), nil
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
	return services.WithValue(ctx, StoreCtxKey, s)
}

func FromCtx(ctx context.Context) Shares {
	s, ok := services.Value(ctx, StoreCtxKey).(Shares)
	if !ok {
		panic("no shares store in context")
	}

	return s
}
