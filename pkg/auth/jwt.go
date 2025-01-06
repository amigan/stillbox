package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"dynatron.me/x/stillbox/pkg/database"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"github.com/rs/zerolog/log"
)

const (
	CookieName = "stillboxJwt"
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

	// PublicRoutes installs the auth route to the provided chi Router.
	PublicRoutes(chi.Router)

	// PublicRoutes installs the refresh route to the provided chi Router.
	PrivateRoutes(chi.Router)
}

type claims map[string]interface{}

func UIDFrom(ctx context.Context) *int32 {
	tok, _, err := jwtauth.FromContext(ctx)
	if err != nil {
		return nil
	}

	uidStr := tok.Subject()
	uidInt, err := strconv.Atoi(uidStr)
	if err != nil {
		return nil
	}

	uid := int32(uidInt)

	return &uid
}

func (a *Auth) Authenticated(r *http.Request) (claims, bool) {
	// TODO: check IP against ACL, or conf.Public, and against map of routes
	tok, cl, err := jwtauth.FromContext(r.Context())
	return cl, err != nil && tok != nil
}

func (a *Auth) VerifyMiddleware() func(http.Handler) http.Handler {
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

func (a *Auth) AuthMiddleware() func(http.Handler) http.Handler {
	return jwtauth.Authenticator(a.jwt)
}

func (a *Auth) initJWT() {
	if string(a.cfg.JWTSecret) == "super secret string" {
		log.Fatal().Msg("JWT secret is the default!")
	}
	a.jwt = jwtauth.New("HS256", []byte(a.cfg.JWTSecret), nil)
}

func (a *Auth) Login(ctx context.Context, username, password string) (token string, err error) {
	q := database.FromCtx(ctx)
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

func (a *Auth) newToken(uid int) string {
	claims := claims{
		"sub": strconv.Itoa(int(uid)),
	}
	jwtauth.SetExpiryIn(claims, time.Hour*24*30) // one month
	_, tokenString, err := a.jwt.Encode(claims)
	if err != nil {
		panic(err)
	}
	return tokenString
}

func (a *Auth) allowInsecureCookie(r *http.Request) bool {
	host := strings.Split(r.Host, ":")
	v, has := a.cfg.AllowInsecure[host[0]]
	return has && v
}

func (a *Auth) setInsecureCookie(cookie *http.Cookie) {
	if a.cfg.SameSiteNoneWhenInsecure {
		cookie.Secure = true
		cookie.SameSite = http.SameSiteNoneMode
	} else {
		cookie.Secure = false
		cookie.SameSite = http.SameSiteLaxMode
	}
}

func (a *Auth) routeRefresh(w http.ResponseWriter, r *http.Request) {
	jwToken, _, err := jwtauth.FromContext(r.Context())
	if err != nil {
		http.Error(w, "Invalid token", http.StatusBadRequest)
		return
	}
	existingSubjectUID := jwToken.Subject()
	if existingSubjectUID == "" {
		http.Error(w, "Invalid token", http.StatusBadRequest)
		return
	}
	uid, err := strconv.Atoi(existingSubjectUID)
	if err != nil {
		log.Error().Str("sub", existingSubjectUID).Err(err).Msg("atoi uid for token refresh")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	tok := a.newToken(uid)

	cookie := &http.Cookie{
		Name:     CookieName,
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
	}

	if a.allowInsecureCookie(r) {
		a.setInsecureCookie(cookie)
	}

	if cookie.Secure {
		cookie.Domain = r.Host
	}
	http.SetCookie(w, cookie)

	jr := struct {
		JWT string `json:"jwt"`
	}{
		JWT: tok,
	}

	render.JSON(w, r, &jr)
}

func (a *Auth) routeAuth(w http.ResponseWriter, r *http.Request) {
	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	var err error

	switch strings.Split(r.Header.Get("Content-Type"), ";")[0] {
	case "application/json":
		err = json.NewDecoder(r.Body).Decode(&creds)
	default:
		err = r.ParseForm()
		if err != nil {
			break
		}
		creds.Username, creds.Password = r.PostFormValue("username"), r.PostFormValue("password")
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if a.rl.RespondOnLimit(w, r, creds.Username) {
		return
	}

	if creds.Username == "" || creds.Password == "" {
		http.Error(w, "blank credentials", http.StatusBadRequest)
		return
	}

	tok, err := a.Login(r.Context(), creds.Username, creds.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	cookie := &http.Cookie{
		Name:     CookieName,
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
	}

	cookie.Domain = r.Host
	if a.allowInsecureCookie(r) {
		a.setInsecureCookie(cookie)
	}

	http.SetCookie(w, cookie)

	jr := struct {
		JWT string `json:"jwt"`
	}{
		JWT: tok,
	}

	render.JSON(w, r, &jr)
}

func (a *Auth) routeLogout(w http.ResponseWriter, r *http.Request) {
	cookie := &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		MaxAge:   -1,
	}

	cookie.Domain = r.Host
	if a.allowInsecureCookie(r) {
		cookie.Secure = true
		cookie.SameSite = http.SameSiteNoneMode
	}

	http.SetCookie(w, cookie)

	jr := struct {
		Message string `json:"message"`
	}{
		Message: "logged out",
	}

	render.JSON(w, r, &jr)
}
