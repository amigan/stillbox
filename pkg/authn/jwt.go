package authn

import (
	"context"
	"net/http"
	"time"

	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/users"

	"github.com/go-chi/jwtauth/v5"
	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/rs/zerolog/log"
)

const (
	CallRealm = "me.dynatron.stillbox.call"
)

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

func (a *authenticator) Authenticated(r *http.Request) (claims, bool) {
	// TODO: check IP against ACL, or conf.Public, and against map of routes
	tok, cl, err := jwtauth.FromContext(r.Context())
	return cl, err != nil && tok != nil
}

func (a *authenticator) VerifyMiddleware() func(http.Handler) http.Handler {
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

func (a *authenticator) SubjectMiddleware(requireToken bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		hfn := func(w http.ResponseWriter, r *http.Request) {
			token, _, err := jwtauth.FromContext(r.Context())

			if err != nil && requireToken {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}

			ctx := r.Context()

			if token != nil {
				err := jwt.Validate(token, a.jwt.ValidateOptions()...)
				if err != nil {
					err = jwtauth.ErrorReason(err)
					http.Error(w, err.Error(), http.StatusUnauthorized)
					return
				}

				var sub entities.Subject

				subjectString := token.Subject()
				realm, hasRealm := token.Get("realm")
				if hasRealm {
					realmStr, ok := realm.(string)
					if !ok {
						log.Error().Msg("realm not set")
						http.Error(w, "realm not set", http.StatusUnauthorized)
						return
					}
					switch realmStr {
					case CallRealm:
						cUUID, err := uuid.Parse(subjectString)
						if err != nil {
							log.Error().Err(err).Msg("cannot parse call UUID")
							http.Error(w, err.Error(), http.StatusUnauthorized)
							return
						}

						sub = &entities.CallSubject{
							CallID: cUUID,
						}
					default:
						log.Error().Str("realm", realmStr).Msg("unknown realm")
						http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
						return
					}
				} else {
					sub, err = users.FromCtx(ctx).GetUser(ctx, subjectString)
					if err != nil {
						log.Error().Str("username", subjectString).Err(err).Msg("subject middleware get subject")
						http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
						return
					}
				}

				ctx = entities.CtxWithSubject(ctx, sub)

				next.ServeHTTP(w, r.WithContext(ctx))

				return
			}

			// Public subject
			ctx = entities.CtxWithSubject(ctx, entities.NewPublicSubject(r))
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

func (a *authenticator) initJWT() {
	if string(a.cfg.JWTSecret) == "super secret string" {
		log.Fatal().Msg("JWT secret is the default!")
	}
	a.jwt = jwtauth.New("HS256", []byte(a.cfg.JWTSecret), nil)
}

func (a *authenticator) NewAccessToken(username string) string {
	claims := claims{
		"sub": username,
	}
	jwtauth.SetExpiryIn(claims, time.Hour*24*30) // one month
	_, tokenString, err := a.jwt.Encode(claims)
	if err != nil {
		panic(err)
	}
	return tokenString
}

func (a *authenticator) NewCallToken(callID string) string {
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
