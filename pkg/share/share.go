package share

import (
	"context"
	"time"

	"dynatron.me/x/stillbox/internal/jsontypes"

	"github.com/google/uuid"
	"github.com/matoous/go-nanoid"
)

const (
	SlugLength = 20
)

type EntityType string

const (
	EntityIncident EntityType = "incident"
	EntityCall     EntityType = "call"
)

// If an incident is shared, all calls that are part of it must be shared too, but this can be through the incident share (/share/bLaH/callID[.mp3])

type Share struct {
	ID         string          `json:"id"`
	Type       EntityType      `json:"entityType"`
	EntityID   uuid.UUID       `json:"entityID"`
	Expiration *jsontypes.Time `json:"expiration"`
}

// NewShare creates a new share.
func (s *service) NewShare(ctx context.Context, shType EntityType, shID uuid.UUID, exp *time.Duration) (id string, err error) {
	id, err = gonanoid.ID(SlugLength)
	if err != nil {
		return
	}

	store := FromCtx(ctx)

	var expT *jsontypes.Time
	if exp != nil {
		tt := time.Now().Add(*exp)
		expT = (*jsontypes.Time)(&tt)
	}

	share := &Share{
		ID:         id,
		Type:       shType,
		EntityID:   shID,
		Expiration: expT,
	}

	err = store.Create(ctx, share)
	if err != nil {
		return
	}

	return id, nil
}
