package authn

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"dynatron.me/x/stillbox/pkg/authz"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/users"

	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"github.com/rs/zerolog/log"
	"github.com/wagslane/go-password-validator"
)

var (
	ErrBadPassword = errors.New("bad password")
)

const (
	CookieName             = "stillboxJwt"
	MinimumPasswordEntropy = 50.0
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

	return a.NewRefreshToken(username)
}

type PasswordValidationErr struct {
	error
}

var ErrPasswordValidation = errors.New("password validation error")

func (p PasswordValidationErr) Unwrap() error {
	return ErrPasswordValidation
}

func (a *authn) ValidatePassword(ctx context.Context, ust users.Store, username, password string) (*users.User, error) {
	user, err := ust.GetUser(ctx, username)
	if err != nil || user == nil {
		log.Error().Str("username", username).Err(err).Msg("getUsers failed")
		_ = bcrypt.CompareHashAndPassword([]byte("thisPreventsTimingAttacks"), []byte(password))
		return user, ErrBadPassword
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return user, ErrBadPassword
	}

	return user, nil
}

func (a *authn) ChangePassword(ctx context.Context, username, oldPassword *string, newPassword string) error {
	ust := users.FromCtx(ctx)

	callerUN := UsernameFrom(ctx)
	if username == nil && callerUN == nil {
		return authz.ErrBadSubject
	}

	if username == nil {
		username = callerUN
	}

	targetUser, err := ust.GetUser(ctx, *username)
	if err != nil {
		return err
	}

	callerSubject, err := authz.Check(ctx, targetUser, authz.WithActions(entities.ActionUpdate))
	if err != nil {
		return err
	}

	callerIsAdmin := entities.HasRole(callerSubject, entities.RoleAdmin)
	// if either we are not an admin, or callerUN is set, and we are changing our own (admin) password
	oldPasswordRequired := !callerIsAdmin || (callerUN != nil && *username == *callerUN)

	if oldPassword == nil && oldPasswordRequired {
		return ErrBadPassword
	}

	if oldPasswordRequired {
		_, err := a.ValidatePassword(ctx, ust, *username, *oldPassword)
		if err != nil {
			return err
		}
	}

	if !callerIsAdmin {
		err = passwordvalidator.Validate(newPassword, MinimumPasswordEntropy)
		if err != nil {
			return PasswordValidationErr{err}
		}
	}

	hashpw, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return ust.ChangePassword(ctx, *username, string(hashpw))
}

func (a *authn) Login(ctx context.Context, username, password, source string) (token string, err error) {
	ust := users.FromCtx(ctx)

	user, err := a.ValidatePassword(ctx, ust, username, password)
	if err != nil {
		if errors.Is(err, ErrBadPassword) {
			err = ErrLoginFailed
		}

		return "", err
	}

	err = ust.RecordLogin(ctx, username, source)
	if err != nil {
		log.Error().Str("username", username).Str("source", source).Err(err).Msg("record login failed")
	}

	return a.NewRefreshToken(user.Username)
}

func (a *authn) routeRefresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	jwToken, claims, err := jwtauth.FromContext(ctx)
	if err != nil {
		a.metrics.FailedTokenRefreshCount.Inc()
		http.Error(w, "Invalid token", http.StatusBadRequest)
		return
	}

	existingSubjectUsername := jwToken.Subject()
	if existingSubjectUsername == "" {
		log.Error().Str("remote", r.RemoteAddr).Msg("no subject in token")
		http.Error(w, "Invalid token", http.StatusBadRequest)
		a.metrics.FailedTokenRefreshCount.Inc()
		return
	}

	iatTime, ok := claims["iat"].(time.Time)
	if !ok {
		log.Error().Str("remote", r.RemoteAddr).Str("username", existingSubjectUsername).Msg("no issuedAt in refresh token")
		http.Error(w, "Invalid token", http.StatusBadRequest)
		a.metrics.FailedTokenRefreshCount.Inc()
		return
	}

	iatTime = iatTime.UTC()

	refreshTok, err := a.Refresh(ctx, existingSubjectUsername, iatTime, r.RemoteAddr)
	if err != nil {
		log.Error().Err(err).Str("username", existingSubjectUsername).Msg("refresh failed")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		a.metrics.FailedTokenRefreshCount.Inc()
		return
	}

	accessTok := a.NewAccessToken(existingSubjectUsername)

	a.metrics.TokenRefreshCount.Inc()

	cookie := &http.Cookie{
		Name:     CookieName,
		Value:    accessTok,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
	}

	domain := strings.Split(r.Host, ":")[0]

	a.SetInsecureCookieIfAllowed(domain, cookie)

	if cookie.Secure {
		cookie.Domain = domain
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
		a.metrics.FailedLoginCount.Inc()
		return
	}

	accessTok := a.NewAccessToken(creds.Username)

	a.metrics.SuccessfulLoginCount.Inc()

	cookie := &http.Cookie{
		Name:     CookieName,
		Value:    accessTok,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		MaxAge:   60 * 60 * 24 * 30, // one month
	}

	domain := strings.Split(r.Host, ":")[0]

	a.SetInsecureCookieIfAllowed(domain, cookie)

	if cookie.Secure {
		cookie.Domain = domain
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

	domain := strings.Split(r.Host, ":")[0]

	a.SetInsecureCookieIfAllowed(domain, cookie)

	if cookie.Secure {
		cookie.Domain = domain
	}

	http.SetCookie(w, cookie)

	jr := struct {
		Message string `json:"message"`
	}{
		Message: "logged out",
	}

	render.JSON(w, r, &jr)
}

// AllowInsecureCookie returns whether the request's host is allowed to use insecure cookies per config.
func (a *authn) AllowInsecureCookie(r *http.Request) bool {
	return a.domainInsecureCookieAllowed(strings.Split(r.Host, ":")[0])
}

func (a *authn) domainInsecureCookieAllowed(domain string) bool {
	v, has := a.cfg.AllowInsecure[domain]
	return has && v
}

func (a *authn) SetInsecureCookieIfAllowed(domain string, cookie *http.Cookie) {
	if a.domainInsecureCookieAllowed(domain) {
		a.setInsecureCookie(cookie)
	}
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
