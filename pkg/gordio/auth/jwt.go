package auth

import (
	"context"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"dynatron.me/x/stillbox/pkg/gordio/database"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"github.com/rs/zerolog/log"
)

type jwtAuth interface {
	// Authenticated returns whether the request is authenticated. It also returns the claims.
	Authenticated(r *http.Request) (claims, bool)

	// Login attempts to return a JWT for the provided user and password.
	Login(ctx context.Context, username, password string) (token string, err error)

	// InstallVerifyMiddleware installs the JWT verifier middleware to the provided chi Router.
	VerifyMiddleware() func(http.Handler) http.Handler

	// InstallAuthMiddleware installs the JWT authenticator middleware to the provided chi Router.
	AuthMiddleware() func(http.Handler) http.Handler

	// InstallRoutes installs the auth route to the provided chi Router.
	PublicRoutes(chi.Router)
}

type claims map[string]interface{}

func (a *authenticator) Authenticated(r *http.Request) (claims, bool) {
	// TODO: check IP against ACL, or conf.Public, and against map of routes
	tok, cl, err := jwtauth.FromContext(r.Context())
	return cl, err != nil && tok != nil
}

func (a *authenticator) VerifyMiddleware() func(http.Handler) http.Handler {
	return jwtauth.Verifier(a.jwt)
}

func (a *authenticator) AuthMiddleware() func(http.Handler) http.Handler {
	return jwtauth.Authenticator(a.jwt)
}

func (a *authenticator) Login(ctx context.Context, username, password string) (token string, err error) {
	q := database.New(database.FromCtx(ctx))
	users, err := q.GetUsers(ctx)
	if err != nil {
		log.Error().Err(err).Msg("getUsers failed")
		return "", ErrLoginFailed
	}

	var found *database.User

	for _, u := range users {
		if u.Username == username {
			found = &u
		}
	}

	if found == nil {
		_ = bcrypt.CompareHashAndPassword([]byte("lol@timing"), []byte(password))
		return "", ErrLoginFailed
	} else {
		err = bcrypt.CompareHashAndPassword([]byte(found.Password), []byte(password))
		if err != nil {
			return "", ErrLoginFailed
		}
	}

	return a.newToken(found.ID), nil
}

func (a *authenticator) newToken(uid int32) string {
	claims := claims{
		"user_id": uid,
	}
	jwtauth.SetExpiryIn(claims, time.Hour*24*30) // one month
	_, tokenString, err := a.jwt.Encode(claims)
	if err != nil {
		panic(err)
	}
	return tokenString
}

func (a *authenticator) PublicRoutes(r chi.Router) {
	r.Post("/login", a.routeAuth)
}

func (a *authenticator) allowInsecureCookie(r *http.Request) bool {
	v, has := a.cfg.AllowInsecure[r.Host]
	return has && v
}

func (a *authenticator) routeAuth(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	username, password := r.PostFormValue("username"), r.PostFormValue("password")
	if username == "" || password == "" {
		http.Error(w, "blank credentials", http.StatusBadRequest)
		return
	}

	tok, err := a.Login(r.Context(), username, password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	cookie := &http.Cookie{
		Name:     "jwt",
		Value:    tok,
		HttpOnly: true,
		Secure:   !a.allowInsecureCookie(r),
	}

	if cookie.Secure {
		cookie.Domain = a.cfg.Domain
	}
	http.SetCookie(w, cookie)

	jr := struct {
		JWT string `json:"jwt"`
	}{
		JWT: tok,
	}

	render.JSON(w, r, &jr)
}
