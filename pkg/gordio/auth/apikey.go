package auth

import (
	"context"
	"time"

	"dynatron.me/x/stillbox/pkg/gordio/database"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

func (a *Authenticator) CheckAPIKey(ctx context.Context, key string) (*database.ApiKey, error) {
	keyUuid, err := uuid.Parse(key)
	if err != nil {
		log.Error().Str("apikey", key).Msg("cannot parse key")
		return nil, ErrBadRequest
	}

	db := database.FromCtx(ctx)
	apik, err := db.GetAPIKey(ctx, keyUuid)
	if err != nil {
		if database.IsNoRows(err) {
			log.Error().Str("apikey", keyUuid.String()).Msg("no such key")
			return nil, ErrUnauthorized
		}

		return nil, ErrInternal
	}

	if (apik.Disabled != nil && *apik.Disabled) || (apik.Expires.Valid && time.Now().After(apik.Expires.Time)) {
		log.Error().Str("key", apik.ApiKey.String()).Msg("key disabled")
		return nil, ErrUnauthorized
	}

	return &apik, nil
}
