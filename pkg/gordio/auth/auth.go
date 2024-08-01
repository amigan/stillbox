package auth

import (
	"errors"
	"net/http"

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
	domain string
	jwt    *jwtauth.JWTAuth
}

// NewAuthenticator creates a new Authenticator with the provided JWT secret and cookie domain.
func NewAuthenticator(jwtSecret string, domain string) Authenticator {
	return &authenticator{
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
