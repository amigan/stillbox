package authn

import (
	"context"
	"net/http"
	"strings"
	"time"

	"dynatron.me/x/stillbox/internal/acl"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/users"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

func (a *authn) initAPIKeyACL(cfg *acl.IPConfig) error {
	a.Lock()
	defer a.Unlock()

	c, err := cfg.IPACL()
	if err != nil {
		return err
	}

	a.apiKeyACL = c

	return nil
}

func (a *authn) apiKeyAPIKeySubject(ctx context.Context, key string) (entities.Subject, error) {
	b64hash := users.APIKeyHash(key)
	apik, err := a.ust.GetAPIKey(ctx, users.APIKeyKindRdio, b64hash)
	if err != nil {
		if database.IsNoRows(err) {
			return nil, ErrUnauthorized
		}

		return nil, err
	}

	if apik.Disabled || (apik.Expires != nil && time.Now().After(*apik.Expires)) {
		return nil, ErrUnauthorized
	}

	return a.ust.GetUser(ctx, apik.Username)
}

func (a *authn) rdioAPIKeySubject(ctx context.Context, key string) (entities.Subject, error) {
	keyUuid, err := uuid.Parse(key)
	if err != nil {
		return nil, err
	}

	b64hash := users.APIKeyHash(keyUuid.String())
	apik, err := a.ust.GetAPIKey(ctx, users.APIKeyKindRdio, b64hash)
	if err != nil {
		if database.IsNoRows(err) {
			return nil, ErrUnauthorized
		}

		return nil, err
	}

	if apik.Disabled || (apik.Expires != nil && time.Now().After(*apik.Expires)) {
		return nil, ErrUnauthorized
	}

	return a.ust.GetUser(ctx, apik.Username)
}

// APIKeyMiddleware validates the API key set in the header and sets the Subject in context with the resolved User.
// This is the normal API key middleware for use everywhere else.
func (a *authn) APIKeyMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		hfn := func(w http.ResponseWriter, r *http.Request) {
			a.Lock()
			aclResult := a.apiKeyACL.Allowed(r)
			a.Unlock()

			if aclResult != nil {
				log.Error().Err(aclResult).Str("remote_addr", r.RemoteAddr).Msg("api key auth ACL check")
				ErrorResponse(w, ErrUnauthorized)
				return
			}

			ctx := r.Context()

			key := r.Header.Values("Authorization")
			if len(key) < 1 {
				ErrorResponse(w, ErrUnauthorized)
				return
			}

			sub, err := a.apiKeyAPIKeySubject(ctx, key[0])
			if err != nil {
				log.Error().Str("key", key[0]).Err(err).Msg("api auth failed")
				ErrorResponse(w, err)
				return
			}

			ctx = entities.CtxWithSubject(ctx, sub)
			next.ServeHTTP(w, r.WithContext(ctx))
		}

		return http.HandlerFunc(hfn)
	}

}

// MultipartAPIKeyMiddleware validates the provided key and sets the Subject in context with the resolved User.
// This is only for use when multipart/form-data is expected. It ideally has one use, and that is for the
// Rdio HTTP source.
func (a *authn) MultipartAPIKeyMiddleware(formKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		hfn := func(w http.ResponseWriter, r *http.Request) {
			a.Lock()
			aclResult := a.apiKeyACL.Allowed(r)
			a.Unlock()

			if aclResult != nil {
				log.Error().Err(aclResult).Str("remote_addr", r.RemoteAddr).Msg("api key auth ACL check")
				ErrorResponse(w, ErrUnauthorized)
				return
			}

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
			sub, err := a.rdioAPIKeySubject(ctx, key)
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
