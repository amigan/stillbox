package shares

import (
	"context"
	"errors"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/authz"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/calls/callstore"
	"dynatron.me/x/stillbox/pkg/incidents/incstore"
	"dynatron.me/x/stillbox/pkg/users"

	"github.com/google/uuid"
	"github.com/matoous/go-nanoid"
)

const (
	SlugLength = 20
)

var (
	ErrExpirationTooSoon = errors.New("expiration too soon")
	ErrBadType           = errors.New("bad share type")
)

type EntityType string

const (
	EntityIncident EntityType = "incident"
	EntityCall     EntityType = "call"
)

func (et EntityType) IsValid() bool {
	switch et {
	case EntityCall, EntityIncident:
		return true
	}

	return false
}

// If an incident is shared, all calls that are part of it must be shared too, but this can be through the incident share (/share/bLaH/callID[.mp3])

type Share struct {
	ID         string          `json:"id"`
	Type       EntityType      `json:"entityType"`
	Date       *jsontypes.Time `json:"entityDate,omitempty"` // we handle this for the user
	Owner      users.UserID    `json:"-"`
	OwnerUser  *string         `json:"owner,omitempty"`
	EntityID   uuid.UUID       `json:"entityID"`
	Expiration *jsontypes.Time `json:"expiration"`
}

func (s *Share) GetName() string {
	return "SHARE:" + s.ID
}

func (s *Share) String() string {
	return s.GetName()
}

func (s *Share) GetRoles() []string {
	return []string{entities.RoleShareGuest}
}

func (s *Share) GetResourceName() string {
	return entities.ResourceShare
}

type CreateShareParams struct {
	Type       EntityType          `json:"entityType"`
	EntityID   uuid.UUID           `json:"entityID"`
	Expiration *jsontypes.Duration `json:"expiration"`
}

func (s *service) checkEntity(ctx context.Context, sh *CreateShareParams) (*time.Time, error) {
	var t *time.Time
	switch sh.Type {
	case EntityCall:
		cs := callstore.FromCtx(ctx)
		// Call does RBAC for us
		call, err := cs.Call(ctx, sh.EntityID)
		if err != nil {
			return nil, err
		}

		t = &call.DateTime
	case EntityIncident:
		is := incstore.FromCtx(ctx)
		i, err := is.Owner(ctx, sh.EntityID)
		if err != nil {
			return nil, err
		}
		_, err = authz.Check(ctx, &i, authz.WithActions(entities.ActionShare))
		if err != nil {
			return nil, err
		}
	}

	return t, nil
}

// NewShare creates a new share.
func (s *service) NewShare(ctx context.Context, sh CreateShareParams) (*Share, error) {
	if !sh.Type.IsValid() {
		return nil, ErrBadType
	}

	// entTime is only meaningful if we are a call
	entTime, err := s.checkEntity(ctx, &sh)
	if err != nil {
		return nil, err
	}

	u, err := users.From(ctx)
	if err != nil {
		return nil, err
	}

	id, err := gonanoid.ID(SlugLength)
	if err != nil {
		return nil, err
	}

	share := &Share{
		ID:       id,
		Type:     sh.Type,
		Date:     (*jsontypes.Time)(entTime),
		Owner:    u.ID,
		EntityID: sh.EntityID,
	}

	if sh.Expiration != nil {
		if sh.Expiration.Duration() < time.Minute {
			return nil, ErrExpirationTooSoon
		}
		share.Expiration = common.PtrTo(jsontypes.Time(time.Now().Add(sh.Expiration.Duration())))
	}

	err = s.Create(ctx, share)
	if err != nil {
		return nil, err
	}

	return share, nil
}
