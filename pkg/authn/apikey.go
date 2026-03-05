package authn

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"dynatron.me/x/stillbox/internal/acl"
	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/authz"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/users"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

var (
	ErrInvalidScopes = errors.New("invalid scope(s)")
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

func (a *authn) rdioAPIKeySubject(ctx context.Context, key string) (entities.Subject, error) {
	keyUuid, err := uuid.Parse(key)
	if err != nil {
		return nil, err
	}

	b64hash := APIKeyHash(keyUuid.String())
	apik, err := a.ust.GetAPIKey(ctx, users.APIKeyKindRdio, &b64hash, nil)
	if err != nil {
		if database.IsNoRows(err) {
			return nil, ErrUnauthorized
		}

		return nil, err
	}

	if apik.Disabled || (apik.Expires != nil && time.Now().After(*apik.Expires)) {
		return nil, ErrUnauthorized
	}

	user, err := a.ust.GetUser(ctx, apik.Username)
	if err != nil {
		return nil, err
	}

	return entities.NewAPIKeySubject(user, entities.ScopeSubmit), nil
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

type CreateAPIKeyRequest struct {
	Owner     *string          `json:"owner"`
	Name      *string          `json:"name"`
	ExpiresAt *jsontypes.Time  `json:"expiresAt"`
	Disabled  bool             `json:"disabled"`
	Kind      users.APIKeyKind `json:"kind"`
	Scopes    []string         `json:"scopes"`
}

func (a *authn) CreateAPIKey(ctx context.Context, rq CreateAPIKeyRequest) (*users.APIKey, error) {
	ust := users.FromCtx(ctx)

	var userID users.UserID
	var username string

	if rq.Owner != nil {
		user, err := ust.GetUser(ctx, *rq.Owner)
		if err != nil {
			return nil, err
		}
		username = user.Username
		userID = user.ID
	} else {
		sub := entities.SubjectFrom(ctx)
		if u, isUser := sub.(*users.User); isUser {
			userID = u.ID
			username = u.Username
		} else {
			return nil, users.ErrNoUIDSpecified
		}
	}

	if !entities.ValidateScopes(rq.Scopes) {
		return nil, ErrInvalidScopes
	}

	ak := &users.APIKey{
		OwnerID:   userID,
		Name:      rq.Name,
		Kind:      rq.Kind,
		CreatedAt: jsontypes.Time(time.Now()),
		Expires:   rq.ExpiresAt,
		Disabled:  rq.Disabled,
		Scopes:    rq.Scopes,
	}

	_, err := authz.Check(ctx, ak, authz.WithActions(entities.ActionCreate))
	if err != nil {
		return nil, err
	}

	switch rq.Kind {
	case users.APIKeyKindRdio:
		key, hashedKey, err := generateRdioAPIKey()
		if err != nil {
			return nil, err
		}

		ak.Key = key
		ak.Hash = &hashedKey
	case users.APIKeyKindAPIKey:
		jwtID := uuid.New()
		ak.JWTID = &jwtID
		key, err := a.NewAPIKeyToken(username, (*time.Time)(ak.Expires), jwtID, ak.Scopes)
		if err != nil {
			return nil, err
		}

		err = ust.CreateAPIKey(ctx, ak)
		if err != nil {
			return nil, err
		}
		// we don't store the final key, this is just to return
		ak.Key = key

		return ak, nil
	default:
		return nil, users.ErrAPIKeyKindInvalid
	}

	return nil, nil
}

func generateRdioAPIKey() (key, hash string, err error) {
	key = uuid.New().String()

	return key, APIKeyHash(key), nil
}

func APIKeyHash(key string) string {
	hash := sha256.Sum256([]byte(key))
	return base64.StdEncoding.EncodeToString(hash[:])
}
