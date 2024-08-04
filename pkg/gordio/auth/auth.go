package auth

import (
	"errors"
	"net/http"

	"dynatron.me/x/stillbox/pkg/gordio/config"
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

type authenticator struct {
	jwt *jwtauth.JWTAuth
	cfg config.Auth
}

// NewAuthenticator creates a new Authenticator with the provided config.
func NewAuthenticator(cfg config.Auth) Authenticator {
	return &authenticator{
		jwt: jwtauth.New("HS256", []byte(cfg.JWTSecret), nil),
		cfg: cfg,
	}
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
