package authn

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/database"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

func (a *authn) apiKeySubject(ctx context.Context, key string) (entities.Subject, error) {
	keyUuid, err := uuid.Parse(key)
	if err != nil {
		return nil, err
	}

	hash := sha256.Sum256([]byte(keyUuid.String()))
	b64hash := base64.StdEncoding.EncodeToString(hash[:])
	apik, err := a.ust.GetAPIKey(ctx, b64hash)
	if err != nil {
		if database.IsNoRows(err) {
			return nil, ErrUnauthorized
		}

		return nil, err
	}

	if (apik.Disabled != nil && *apik.Disabled) || (apik.Expires.Valid && time.Now().After(apik.Expires.Time)) {
		return nil, ErrUnauthorized
	}

	return a.ust.GetUser(ctx, apik.Username)
}

// CheckAPIKey validates the provided key and returns the API owner's users.UserID.
// An error is returned if validation fails for any reason.
func (a *authn) APIKeyMiddleware(formKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		hfn := func(w http.ResponseWriter, r *http.Request) {
			if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "multipart/form-data" {
				ErrorResponse(w, ErrBadRequest)
				return
			}

			err := r.ParseMultipartForm(1024 * 1024 * 2) // 2MB
			if err != nil {
				ErrorResponse(w, ErrBadRequest)
				return
			}

			ctx := r.Context()

			key := r.Form.Get(formKey)
			sub, err := a.apiKeySubject(ctx, key)
			if err != nil {
				log.Error().Str("key", key).Err(err).Msg("api auth failed")
				ErrorResponse(w, err)
				return
			}

			ctx = entities.CtxWithSubject(ctx, sub)
			next.ServeHTTP(w, r.WithContext(ctx))
		}

		return http.HandlerFunc(hfn)
	}
}
