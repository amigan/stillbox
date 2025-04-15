package authn

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"time"

	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/database"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type apiKeyAuth interface {
	// CheckAPIKey validates the provided key and returns the API owner's users.UserID.
	// An error is returned if validation fails for any reason.
	CheckAPIKey(ctx context.Context, key string) (entities.Subject, error)
}

func (a *authenticator) CheckAPIKey(ctx context.Context, key string) (entities.Subject, error) {
	keyUuid, err := uuid.Parse(key)
	if err != nil {
		log.Error().Str("apikey", key).Msg("cannot parse key")
		return nil, ErrBadRequest
	}

	hash := sha256.Sum256([]byte(keyUuid.String()))
	b64hash := base64.StdEncoding.EncodeToString(hash[:])
	apik, err := a.ust.GetAPIKey(ctx, b64hash)
	if err != nil {
		if database.IsNoRows(err) {
			log.Error().Str("apikey", keyUuid.String()).Msg("no such key")
			return nil, ErrUnauthorized
		}

		log.Error().Str("apikey", keyUuid.String()).Err(err).Msg("error looking up key")
		return nil, ErrInternal
	}

	if (apik.Disabled != nil && *apik.Disabled) || (apik.Expires.Valid && time.Now().After(apik.Expires.Time)) {
		log.Error().Str("key", apik.ApiKey).Msg("key disabled")
		return nil, ErrUnauthorized
	}

	return a.ust.GetUser(ctx, apik.Username)
}
