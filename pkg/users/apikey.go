package users

import (
	"errors"
	"strings"

	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"github.com/google/uuid"
)

type APIKey struct {
	OwnerID   UserID          `json:"ownerID"`
	Name      *string         `json:"name"`
	Kind      APIKeyKind      `json:"kind"`
	CreatedAt jsontypes.Time  `json:"createdAt"`
	Expires   *jsontypes.Time `json:"expires,omitempty"`
	Disabled  bool            `json:"disabled"`
	Scopes    []string        `json:"scopes,omitempty"`
	Key       string          `json:"key,omitzero"`
	JWTID     *uuid.UUID      `json:"-"`
	Hash      *string         `json:"-"`
}

type APIKeyKind int

const (
	APIKeyKindRdio APIKeyKind = iota + 1
	APIKeyKindAPIKey
)

var (
	ErrAPIKeyKindInvalid = errors.New("invalid API key kind")
	ErrAPIKeyExpired     = errors.New("API key expired or disabled")
)

func (k *APIKeyKind) UnmarshalText(t []byte) error {
	return k.Parse(string(t))
}

func (k *APIKeyKind) Parse(s string) error {
	switch strings.ToLower(s) {
	case "rdio":
		*k = APIKeyKindRdio
	case "apikey":
		*k = APIKeyKindAPIKey
	default:
		return ErrAPIKeyKindInvalid
	}

	return nil
}

func (k *APIKeyKind) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	return k.Parse(s)
}

func (k *APIKeyKind) MarshalJSON() ([]byte, error) {
	return []byte(`"` + k.String() + `"`), nil
}

func (k *APIKeyKind) MarshalText() ([]byte, error) {
	return []byte(k.String()), nil
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
