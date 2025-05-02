package server

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/http"

	"dynatron.me/x/stillbox/pkg/authn"
	"dynatron.me/x/stillbox/pkg/settings"
	"github.com/go-chi/jwtauth/v5"
	"github.com/gorilla/csrf"
)

const (
	CSRFKeySetting = "csrf.stillbox"
)

func genCSRFKey() string {
	result := ""
	for {
		if len(result) >= 32 {
			return result
		}
		num, err := rand.Int(rand.Reader, big.NewInt(int64(127)))
		if err != nil {
			panic(err)
		}
		n := num.Int64()
		if n > 32 && n < 127 {
			result += string(rune(n))
		}
	}
}

func (s *Server) CSRFMiddleware(ctx context.Context) (func(http.Handler) http.Handler, error) {
	var key string
	keySetting, err := s.settings.Get(ctx, CSRFKeySetting)
	if err != nil {
		if !errors.Is(err, settings.ErrNoSetting) {
			return nil, err
		} else {
			key = genCSRFKey()
			err := s.settings.Set(ctx, CSRFKeySetting, key)
			if err != nil {
				return nil, err
			}
		}
	} else {
		var isString bool
		key, isString = keySetting.(string)
		if !isString {
			return nil, fmt.Errorf("csrf key setting is not a string")
		}
	}

	mw := csrf.Protect([]byte(key),
		csrf.CookieName("_csrf"),
		csrf.FieldName("_csrf"),
		csrf.TrustedOrigins(s.conf.Server.CORS.AllowedOrigins))

	// for AllowInsecureFor hosts
	insecureMW := csrf.Protect([]byte(key),
		csrf.CookieName("_csrf"),
		csrf.FieldName("_csrf"),
		csrf.Secure(false),
		csrf.TrustedOrigins(s.conf.Server.CORS.AllowedOrigins))

	return func(next http.Handler) http.Handler {
		hfn := func(w http.ResponseWriter, r *http.Request) {
			sbt, ok := r.Context().Value(jwtauth.TokenCtxKey).(authn.Token)
			if ok && sbt != nil && sbt.FromHeader() {
				next.ServeHTTP(w, r)
				return
			}

			if s.auth.AllowInsecureCookie(r) {
				insecureMW(next).ServeHTTP(w, r)
			} else {
				mw(next).ServeHTTP(w, r)
			}
		}

		return http.HandlerFunc(hfn)
	}, nil
}
