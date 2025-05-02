package authn

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"dynatron.me/x/stillbox/pkg/users"

	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"github.com/rs/zerolog/log"
)

const (
	CookieName = "stillboxJwt"
)

func (a *authn) Refresh(ctx context.Context, username string, refreshIAT time.Time, source string) (token string, err error) {
	ust := users.FromCtx(ctx)
	user, err := ust.GetUser(ctx, username)
	if err != nil || user == nil {
		return "", ErrUnauthorized
	}

	if refreshIAT.Before(user.PasswordSetAt.UTC()) {
		log.Error().Str("remote", source).Str("username", username).Time("iat", refreshIAT).Time("passwdSetAt", user.PasswordSetAt.UTC()).Msg("token is from before last password reset")
		return "", ErrUnauthorized
	}

	err = ust.RecordLogin(ctx, username, source)
	if err != nil {
		log.Error().Str("username", username).Str("source", source).Err(err).Msg("record refresh failed")
	}

	return a.NewRefreshToken(username), nil
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

	return a.NewRefreshToken(user.Username), nil
}

func (a *authn) routeRefresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	jwToken, claims, err := jwtauth.FromContext(ctx)
	if err != nil {
		a.metrics.FailedTokenRefreshes.Inc()
		http.Error(w, "Invalid token", http.StatusBadRequest)
		return
	}

	existingSubjectUsername := jwToken.Subject()
	if existingSubjectUsername == "" {
		log.Error().Str("remote", r.RemoteAddr).Msg("no subject in token")
		http.Error(w, "Invalid token", http.StatusBadRequest)
		a.metrics.FailedTokenRefreshes.Inc()
		return
	}

	iatTime, ok := claims["iat"].(time.Time)
	if !ok {
		log.Error().Str("remote", r.RemoteAddr).Str("username", existingSubjectUsername).Msg("no issuedAt in refresh token")
		http.Error(w, "Invalid token", http.StatusBadRequest)
		a.metrics.FailedTokenRefreshes.Inc()
		return
	}

	iatTime = iatTime.UTC()

	refreshTok, err := a.Refresh(ctx, existingSubjectUsername, iatTime, r.RemoteAddr)
	if err != nil {
		log.Error().Err(err).Str("username", existingSubjectUsername).Msg("refresh failed")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		a.metrics.FailedTokenRefreshes.Inc()
		return
	}

	accessTok := a.NewAccessToken(existingSubjectUsername)

	a.metrics.TokenRefreshes.Inc()

	cookie := &http.Cookie{
		Name:     CookieName,
		Value:    accessTok,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
	}

	if a.AllowInsecureCookie(r) {
		a.setInsecureCookie(cookie)
	}

	if cookie.Secure {
		cookie.Domain = strings.Split(r.Host, ":")[0]
	}
	http.SetCookie(w, cookie)

	jr := struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	}{
		AccessToken:  accessTok,
		RefreshToken: refreshTok,
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

	refreshTok, err := a.Login(r.Context(), creds.Username, creds.Password, r.RemoteAddr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		a.metrics.FailedLogins.Inc()
		return
	}

	accessTok := a.NewAccessToken(creds.Username)

	a.metrics.SuccessfulLogins.Inc()

	cookie := &http.Cookie{
		Name:     CookieName,
		Value:    accessTok,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		MaxAge:   60 * 60 * 24 * 30, // one month
	}

	cookie.Domain = strings.Split(r.Host, ":")[0]
	if a.AllowInsecureCookie(r) {
		a.setInsecureCookie(cookie)
	}

	http.SetCookie(w, cookie)

	jr := struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	}{
		AccessToken:  accessTok,
		RefreshToken: refreshTok,
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
	if a.AllowInsecureCookie(r) {
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

func (a *authn) AllowInsecureCookie(r *http.Request) bool {
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
