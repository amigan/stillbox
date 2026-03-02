package authn

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/users"

	"github.com/go-chi/jwtauth/v5"
	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/rs/zerolog/log"
)

const (
	RefreshRealm = "me.dynatron.stillbox.refresh"
	AccessRealm  = "me.dynatron.stillbox.access"
	APIKeyRealm  = "me.dynatron.stillbox.apiKey"
)

var (
	ErrBadRealm = errors.New("bad realm")
)

type jwtAuthenticator struct {
	sync.Mutex // protects jwt
	jwt        *jwtauth.JWTAuth

	refreshExpiry, accessExpiry time.Duration
}

type claims map[string]any

// UsernameFrom gets the username (just the subject from token) from ctx.
func UsernameFrom(ctx context.Context) *string {
	tok, _, err := jwtauth.FromContext(ctx)
	if err != nil {
		return nil
	}

	username := tok.Subject()

	return &username
}

func VerifyRequest(ja *jwtauth.JWTAuth, r *http.Request) (Token, error) {
	sbToken := new(token)
	tokenString := jwtauth.TokenFromHeader(r)
	if tokenString != "" {
		sbToken.fromHeader = true
	} else {
		tokenString = tokenFromCookie(r)
	}

	if tokenString == "" {
		return nil, jwtauth.ErrNoTokenFound
	}

	jt, err := jwtauth.VerifyToken(ja, tokenString)
	if err != nil {
		return nil, err
	}

	sbToken.Token = jt

	return sbToken, nil
}

type tokenFromHeaderKey string

const TokenFromHeaderKey tokenFromHeaderKey = "tokenFromHeader"

type Token interface {
	jwt.Token
	FromHeader() bool
}

type token struct {
	jwt.Token
	fromHeader bool
}

func (t *token) FromHeader() bool {
	return t.fromHeader
}

func (a *jwtAuthenticator) VerifyMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		hfn := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			token, err := VerifyRequest(a.jwt, r)
			ctx = jwtauth.NewContext(ctx, token, err)

			next.ServeHTTP(w, r.WithContext(ctx))
		}
		return http.HandlerFunc(hfn)
	}
}

func tokenFromCookie(r *http.Request) string {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (a *jwtAuthenticator) AuthenticateJWT(ctx context.Context, r *http.Request) (entities.Subject, error) {
	token, _, err := jwtauth.FromContext(r.Context())
	if err != nil {
		return nil, err
	}

	a.Lock()
	err = jwt.Validate(token, a.jwt.ValidateOptions()...)
	a.Unlock()
	if err != nil {
		err = jwtauth.ErrorReason(err)
		return nil, err
	}

	subjectString := token.Subject()

	var realmStr string
	realm, hasRealm := token.Get("realm")
	if hasRealm {
		var ok bool
		realmStr, ok = realm.(string)
		if !ok {
			return nil, ErrBadRealm
		}
	}

	ust := users.FromCtx(ctx)

	switch realmStr {
	case "", RefreshRealm, AccessRealm:
		return ust.GetUser(ctx, subjectString)
	case APIKeyRealm:
		user, err := ust.GetUser(ctx, subjectString)
		if err != nil {
			return nil, err
		}

		var scopes []string
		if sc, has := token.Get("scope"); has {
			switch scp := sc.(type) {
			case []any:
				for _, scope := range scp {
					if str, isStr := scope.(string); isStr {
						scopes = append(scopes, str)
					}
				}
			case any:
				if str, isStr := scp.(string); isStr {
					scopes = strings.Split(str, " ")
				}
			}
		}

		return entities.NewAPIKeySubject(user, scopes...), nil
	default:
		return nil, ErrBadRealm
	}
}

func (a *jwtAuthenticator) Init(cfg config.Auth) {
	if cfg.JWTSecret == "super secret string" {
		log.Fatal().Msg("JWT secret is the default!")
	}
	a.Lock()
	defer a.Unlock()
	a.refreshExpiry = 24 * 30 * time.Hour
	a.accessExpiry = time.Hour

	if cfg.RefreshExpiry != nil {
		a.refreshExpiry = *cfg.RefreshExpiry
	}
	if cfg.AccessExpiry != nil {
		a.accessExpiry = *cfg.AccessExpiry
	}

	if a.accessExpiry > a.refreshExpiry {
		log.Fatal().Dur("accessExpiry", a.accessExpiry).Dur("refreshExpiry", a.refreshExpiry).Msg("access token expiry is longer than refresh token expiry")
	}

	a.jwt = jwtauth.New("HS256", []byte(cfg.JWTSecret), nil)
}

func (a *jwtAuthenticator) NewAccessToken(username string) string {
	claims := claims{
		"sub":   username,
		"realm": AccessRealm,
	}
	jwtauth.SetIssuedNow(claims)
	jwtauth.SetExpiryIn(claims, a.accessExpiry)

	a.Lock()
	defer a.Unlock()
	_, tokenString, err := a.jwt.Encode(claims)
	if err != nil {
		panic(err)
	}
	return tokenString
}

func (a *jwtAuthenticator) NewRefreshToken(username string) (string, error) {
	claims := claims{
		"sub":   username,
		"realm": RefreshRealm,
	}

	jwtauth.SetIssuedNow(claims)
	jwtauth.SetExpiryIn(claims, a.refreshExpiry)

	a.Lock()
	defer a.Unlock()
	_, tokenString, err := a.jwt.Encode(claims)

	return tokenString, err
}

func (a *jwtAuthenticator) NewAPIKeyToken(username string, expires *time.Time, keyID uuid.UUID, scopes []string) (string, error) {
	claims := claims{
		"sub":   username,
		"realm": APIKeyRealm,
		"scope": scopes,
		"jti":   keyID.String(),
	}

	jwtauth.SetIssuedNow(claims)
	if expires != nil {
		jwtauth.SetExpiry(claims, *expires)
	}

	a.Lock()
	defer a.Unlock()
	_, tokenString, err := a.jwt.Encode(claims)

	return tokenString, err
}
