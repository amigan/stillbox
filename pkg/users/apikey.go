package users

import (
	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/authz/entities"
)

type APIKey struct {
	OwnerID   UserID          `json:"ownerID"`
	Name      *string         `json:"name"`
	CreatedAt jsontypes.Time  `json:"createdAt"`
	Expires   *jsontypes.Time `json:"expires,omitempty"`
	Disabled  bool            `json:"disabled"`
	Key       *jsontypes.UUID `json:"key,omitempty"`
	Hash      string          `json:"-"`
}

func (*APIKey) GetResourceName() string {
	return entities.ResourceAPIKey
}
