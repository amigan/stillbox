package auth

import (
	"errors"
	"net/http"

	_ "embed"

	"dynatron.me/x/stillbox/pkg/config"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
)

type UserID int

func (u *UserID) Int32Ptr() *int32 {
	if u == nil {
		return nil
	}

	i := int32(*u)

	return &i
}

// Authenticator performs API key and user JWT authentication.
type Authenticator interface {
	jwtAuth
	apiKeyAuth
}

type Auth struct {
	jwt *jwtauth.JWTAuth
	cfg config.Auth
}

// NewAuthenticator creates a new Authenticator with the provided config.
func NewAuthenticator(cfg config.Auth) *Auth {
	a := &Auth{
		cfg: cfg,
	}
	a.initJWT()

	return a
}

func (a *Auth) HUP(cfg *config.Config) {
	a.cfg = cfg.Auth
	a.initJWT()
}

var (
	ErrLoginFailed  = errors.New("Login failed")
	ErrInternal     = errors.New("Internal server error")
	ErrUnauthorized = errors.New("Unauthorized")
	ErrBadRequest   = errors.New("Bad request")
)

// ErrorResponse writes the error and appropriate HTTP response code.
func ErrorResponse(w http.ResponseWriter, err error) {
	switch err {
	case ErrLoginFailed, ErrUnauthorized:
		http.Error(w, err.Error(), http.StatusUnauthorized)
	case ErrBadRequest:
		http.Error(w, err.Error(), http.StatusBadRequest)
	case ErrInternal:
		fallthrough
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *Auth) PublicRoutes(r chi.Router) {
	r.Post("/api/login", a.routeAuth)
	r.Get("/api/login", a.routeLogin)
}

func (a *Auth) PrivateRoutes(r chi.Router) {
	r.Get("/refresh", a.routeRefresh)
}

//go:embed login.html
var loginPage []byte

func (a *Auth) routeLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	_, _ = w.Write(loginPage)
}
