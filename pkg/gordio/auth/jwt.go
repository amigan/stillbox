package auth

import (
	"context"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"time"

	"dynatron.me/x/stillbox/pkg/gordio/database"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"github.com/rs/zerolog/log"
)

type claims map[string]interface{}

func (a *Authenticator) Authenticated(r *http.Request) (claims, bool) {
	// TODO: check IP against ACL, or conf.Public, and against map of routes
	tok, cl, err := jwtauth.FromContext(r.Context())
	return cl, err != nil && tok != nil
}

func (a *Authenticator) InstallVerifyMiddleware(r chi.Router) {
	r.Use(jwtauth.Verifier(a.jwt))
}

func (a *Authenticator) InstallAuthMiddleware(r chi.Router) {
	r.Use(jwtauth.Authenticator(a.jwt))
}

func (a *Authenticator) Login(ctx context.Context, username, password string) (token string, err error) {
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

	return a.NewToken(found.ID), nil
}

func (a *Authenticator) NewToken(uid int32) string {
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

func (a *Authenticator) InstallRoutes(r chi.Router) {
	r.Post("/auth", a.routeAuth)
}

func (a *Authenticator) routeAuth(w http.ResponseWriter, r *http.Request) {
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
	http.SetCookie(w, &http.Cookie{
		Name:     "jwt",
		Value:    tok,
		HttpOnly: true,
		Secure:   true,
		Domain:   a.domain,
	})

	jr := struct {
		JWT string `json:"jwt"`
	}{
		JWT: tok,
	}

	render.JSON(w, r, &jr)
}
