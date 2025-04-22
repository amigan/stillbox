package authn

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"dynatron.me/x/stillbox/pkg/users"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"github.com/rs/zerolog/log"
)

const (
	CookieName = "stillboxJwt"
)

type loginJWTAuth interface {
	// Authenticated returns whether the request is authenticated. It also returns the claims.
	Authenticated(r *http.Request) (claims, bool)

	// Login attempts to return a JWT for the provided user and password.
	Login(ctx context.Context, username, password, source string) (token string, err error)

	// Refresh ensures the subject is still valid and records a login.
	Refresh(ctx context.Context, username, source string) (token string, err error)

	// InstallVerifyMiddleware installs the JWT verifier middleware to the provided chi Router.
	VerifyMiddleware() func(http.Handler) http.Handler

	// SubjectMiddleware sets the request context subject from JWT or public.
	SubjectMiddleware(requireAuth bool) func(http.Handler) http.Handler

	// PublicRoutes installs the auth route to the provided chi Router.
	PublicRoutes(chi.Router)

	// PublicRoutes installs the refresh route to the provided chi Router.
	PrivateRoutes(chi.Router)
}

func (a *authn) Refresh(ctx context.Context, username, source string) (token string, err error) {
	ust := users.FromCtx(ctx)
	user, err := ust.GetUser(ctx, username)
	if err != nil || user == nil {
		return "", ErrUnauthorized
	}

	err = ust.RecordLogin(ctx, username, source)
	if err != nil {
		log.Error().Str("username", username).Str("source", source).Err(err).Msg("record refresh failed")
	}

	return a.NewAccessToken(username), nil
}

func (a *authn) Login(ctx context.Context, username, password, source string) (token string, err error) {
	ust := users.FromCtx(ctx)
	user, err := ust.GetUser(ctx, username)
	if err != nil || user == nil {
		log.Error().Str("username", username).Err(err).Msg("getUsers failed")
		_ = bcrypt.CompareHashAndPassword([]byte("thisPreventsTimingAttacks"), []byte(password))
		return "", ErrLoginFailed
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return "", ErrLoginFailed
	}

	err = ust.RecordLogin(ctx, username, source)
	if err != nil {
		log.Error().Str("username", username).Str("source", source).Err(err).Msg("record login failed")
	}

	return a.NewAccessToken(user.Username), nil
}

func (a *authn) routeRefresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	jwToken, _, err := jwtauth.FromContext(ctx)
	if err != nil {
		http.Error(w, "Invalid token", http.StatusBadRequest)
		return
	}

	// XXX: WE MUST CHECK IF THE SUBJECT IS STILL VALID

	existingSubjectUsername := jwToken.Subject()
	if existingSubjectUsername == "" {
		http.Error(w, "Invalid token", http.StatusBadRequest)
		return
	}

	tok, err := a.Refresh(ctx, existingSubjectUsername, r.RemoteAddr)
	if err != nil {
		log.Error().Err(err).Str("username", existingSubjectUsername).Msg("refresh failed")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

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
		cookie.Domain = strings.Split(r.Host, ":")[0]
	}
	http.SetCookie(w, cookie)

	jr := struct {
		JWT string `json:"jwt"`
	}{
		JWT: tok,
	}

	render.JSON(w, r, &jr)
}

func (a *authn) routeLogin(w http.ResponseWriter, r *http.Request) {
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

	tok, err := a.Login(r.Context(), creds.Username, creds.Password, r.RemoteAddr)
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
		MaxAge:   60 * 60 * 24 * 30, // one month
	}

	cookie.Domain = strings.Split(r.Host, ":")[0]
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

func (a *authn) routeLogout(w http.ResponseWriter, r *http.Request) {
	cookie := &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		MaxAge:   -1,
	}

	cookie.Domain = strings.Split(r.Host, ":")[0]
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

func (a *authn) allowInsecureCookie(r *http.Request) bool {
	host := strings.Split(r.Host, ":")
	v, has := a.cfg.AllowInsecure[host[0]]
	return has && v
}

func (a *authn) setInsecureCookie(cookie *http.Cookie) {
	if a.cfg.SameSiteNoneWhenInsecure {
		cookie.Secure = true
		cookie.SameSite = http.SameSiteNoneMode
	} else {
		cookie.Secure = false
		cookie.SameSite = http.SameSiteLaxMode
	}
}
