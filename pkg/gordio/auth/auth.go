package auth

import (
	"errors"
	"net/http"

	"github.com/go-chi/jwtauth/v5"
)

// Authenticator performs API key and user JWT authentication.
type Authenticator struct {
	domain string
	jwt    *jwtauth.JWTAuth
}

// NewAuthenticator creates a new Authenticator with the provided JWT secret and cookie domain.
func NewAuthenticator(jwtSecret string, domain string) *Authenticator {
	return &Authenticator{
		domain: domain,
		jwt:    jwtauth.New("HS256", []byte(jwtSecret), nil),
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
