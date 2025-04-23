package authn

import (
	"context"
	"errors"
	"net/http"
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
	CallRealm = "me.dynatron.stillbox.call"
)

var (
	ErrBadRealm = errors.New("bad realm")
)

type jwtAuthenticator struct {
	jwt *jwtauth.JWTAuth
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

func (a *jwtAuthenticator) VerifyMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		hfn := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			token, err := jwtauth.VerifyRequest(a.jwt, r, jwtauth.TokenFromHeader, TokenFromCookie)
			ctx = jwtauth.NewContext(ctx, token, err)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
		return http.HandlerFunc(hfn)
	}
}

func TokenFromCookie(r *http.Request) string {
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

	err = jwt.Validate(token, a.jwt.ValidateOptions()...)
	if err != nil {
		err = jwtauth.ErrorReason(err)
		return nil, err
	}

	var sub entities.Subject

	subjectString := token.Subject()
	realm, hasRealm := token.Get("realm")
	if hasRealm {
		realmStr, ok := realm.(string)
		if !ok {
			return nil, ErrBadRealm
		}

		switch realmStr {
		case CallRealm:
			cUUID, err := uuid.Parse(subjectString)
			if err != nil {
				return nil, err
			}

			sub = &entities.CallSubject{
				CallID: cUUID,
			}
		default:
			return nil, ErrBadRealm
		}
	} else {
		sub, err = users.FromCtx(ctx).GetUser(ctx, subjectString)
		if err != nil {
			return nil, err
		}
	}

	return sub, nil
}

func (a *jwtAuthenticator) Init(cfg config.Auth) {
	if string(cfg.JWTSecret) == "super secret string" {
		log.Fatal().Msg("JWT secret is the default!")
	}
	a.jwt = jwtauth.New("HS256", []byte(cfg.JWTSecret), nil)
}

func (a *jwtAuthenticator) NewAccessToken(username string) string {
	claims := claims{
		"sub": username,
	}
	jwtauth.SetExpiryIn(claims, time.Hour)
	_, tokenString, err := a.jwt.Encode(claims)
	if err != nil {
		panic(err)
	}
	return tokenString
}

func (a *jwtAuthenticator) NewRefreshToken(username string) string {
	claims := claims{
		"sub": username,
	}

	jwtauth.SetIssuedNow(claims)
	jwtauth.SetExpiryIn(claims, time.Hour*24*7) // seven days

	_, tokenString, err := a.jwt.Encode(claims)
	if err != nil {
		panic(err)
	}

	return tokenString
}

func (a *jwtAuthenticator) NewCallToken(callID string) string {
	claims := claims{
		"sub":   callID,
		"realm": CallRealm,
	}
	jwtauth.SetExpiryIn(claims, time.Hour)
	_, tokenString, err := a.jwt.Encode(claims)
	if err != nil {
		panic(err)
	}

	return tokenString
}
