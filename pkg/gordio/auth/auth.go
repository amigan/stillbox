package auth

import (
	"errors"
	"net/http"

	"github.com/go-chi/jwtauth/v5"
)

type Authenticator struct {
	domain string
	jwt    *jwtauth.JWTAuth
}

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
