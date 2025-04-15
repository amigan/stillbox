package authn

import (
	"errors"
	"net/http"
	"time"

	_ "embed"

	"dynatron.me/x/stillbox/pkg/authz"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/users"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"
	"github.com/go-chi/jwtauth/v5"
)

// Authenticator performs API key and user JWT authentication.
type Authenticator interface {
	HUP(*config.Config)
	loginJWTAuth
	apiKeyAuth
	AuthorizedSubjectMiddleware() func(http.Handler) http.Handler
	PublicSubjectMiddleware() func(http.Handler) http.Handler
	NewCallToken(callID string) string
	NewAccessToken(username string) string
}

type authenticator struct {
	rl  *httprate.RateLimiter
	jwt *jwtauth.JWTAuth
	ust users.Store
	cfg config.Auth
}

// NewAuthenticator creates a new Authenticator with the provided config.
func NewAuthenticator(cfg config.Auth, ust users.Store) *authenticator {
	a := &authenticator{
		rl:  httprate.NewRateLimiter(5, 5*time.Minute),
		cfg: cfg,
		ust: ust,
	}
	a.initJWT()

	return a
}

func (a *authenticator) HUP(cfg *config.Config) {
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
	case ErrLoginFailed, ErrUnauthorized, authz.ErrBadSubject:
		http.Error(w, err.Error(), http.StatusUnauthorized)
	case ErrBadRequest:
		http.Error(w, err.Error(), http.StatusBadRequest)
	case ErrInternal:
		fallthrough
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *authenticator) PublicRoutes(r chi.Router) {
	r.Post("/api/login", a.routeLogin)
	r.Get("/api/login", a.routeLoginPage)
}

func (a *authenticator) PrivateRoutes(r chi.Router) {
	r.Get("/api/refresh", a.routeRefresh)
	r.Get("/api/logout", a.routeLogout)
}

//go:embed login.html
var loginPage []byte

func (a *authenticator) routeLoginPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	_, _ = w.Write(loginPage)
}

func (a *authenticator) PublicSubjectMiddleware() func(http.Handler) http.Handler {
	return a.SubjectMiddleware(false)
}

func (a *authenticator) AuthorizedSubjectMiddleware() func(http.Handler) http.Handler {
	return a.SubjectMiddleware(true)
}
