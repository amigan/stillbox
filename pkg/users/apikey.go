package users

import (
	"errors"
	"strings"

	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/authz/entities"
)

type APIKey struct {
	OwnerID   UserID          `json:"ownerID"`
	Name      *string         `json:"name"`
	Kind      APIKeyKind      `json:"kind"`
	CreatedAt jsontypes.Time  `json:"createdAt"`
	Expires   *jsontypes.Time `json:"expires,omitempty"`
	Disabled  bool            `json:"disabled"`
	Key       string          `json:"key,omitzero"`
	Hash      string          `json:"-"`
}

type APIKeyKind int

const (
	APIKeyKindRdio APIKeyKind = iota + 1
	APIKeyKindAPIKey
)

var ErrAPIKeyKindInvalid = errors.New("invalid API key kind")

func (k *APIKeyKind) UnmarshalText(t []byte) error {
	switch strings.ToLower(string(t)) {
	case "rdio":
		*k = APIKeyKindRdio
	case "apikey":
		*k = APIKeyKindAPIKey
	default:
		return ErrAPIKeyKindInvalid
	}

	return nil
}

func (k APIKeyKind) String() string {
	return map[APIKeyKind]string{
		APIKeyKindAPIKey: "apikey",
		APIKeyKindRdio:   "rdio",
	}[k]
}

func (k APIKeyKind) Valid() bool {
	switch k {
	case APIKeyKindAPIKey, APIKeyKindRdio:
		return true
	}

	return false
}

func (*APIKey) GetResourceName() string {
	return entities.ResourceAPIKey
}
