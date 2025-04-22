package authn

import (
	"context"
	_ "embed"
	"errors"
	"net/http"
	"time"

	"dynatron.me/x/stillbox/pkg/authz"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/users"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"
)

// Authn performs API key and user JWT authentication.
type Authn interface {
	HUP(*config.Config)
	AuthorizedSubjectMiddleware() func(http.Handler) http.Handler
	PublicSubjectMiddleware() func(http.Handler) http.Handler
	NewCallToken(callID string) string
	NewAccessToken(username string) string
	VerifyMiddleware() func(http.Handler) http.Handler
	APIKeyMiddleware(formKey string) func(http.Handler) http.Handler
	PrivateRoutes(r chi.Router)
	PublicRoutes(r chi.Router)
}

type authn struct {
	jwtAuthenticator
	rl  *httprate.RateLimiter
	cfg config.Auth
	ust users.Store
}

type Authenticator interface {
	Init(cfg config.Auth)
	Authenticate(context.Context, *http.Request) (entities.Subject, error)
	VerifyMiddleware() func(http.Handler) http.Handler
}

type Provider interface {
	Init() error
	// Authenticated returns whether a request is authenticated, and any claims resulting.
	Authenticated(r *http.Request) (claims, bool)
	PublicRoutes() http.Handler
}

func NewAuthn(cfg config.Auth, ust users.Store) (*authn, error) {
	a := &authn{
		rl:  httprate.NewRateLimiter(5, 5*time.Minute),
		cfg: cfg,
		ust: ust,
	}
	a.jwtAuthenticator.Init(cfg)
	return a, nil
}

func (a *authn) HUP(cfg *config.Config) {
	a.jwtAuthenticator.Init(cfg.Auth)
}

func (a *authn) SubjectMiddleware(requireToken bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		hfn := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			subj, err := a.AuthenticateJWT(ctx, r)
			if err == nil {
				next.ServeHTTP(w, r.WithContext(entities.CtxWithSubject(ctx, subj)))
				return
			} else if requireToken {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}

			// Public subject
			ctx = entities.CtxWithSubject(ctx, entities.NewPublicSubject(r))
			next.ServeHTTP(w, r.WithContext(ctx))
		}
		return http.HandlerFunc(hfn)
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
	if authz.IsErrAccessDenied(err) != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
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

func (a *authn) PublicRoutes(r chi.Router) {
	r.Post("/api/login", a.routeLogin)
	r.Get("/api/login", a.routeLoginPage)
}

func (a *authn) PrivateRoutes(r chi.Router) {
	r.Get("/api/refresh", a.routeRefresh)
	r.Get("/api/logout", a.routeLogout)
}

//go:embed login.html
var loginPage []byte

func (a *authn) routeLoginPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	_, _ = w.Write(loginPage)
}

func (a *authn) PublicSubjectMiddleware() func(http.Handler) http.Handler {
	return a.SubjectMiddleware(false)
}

func (a *authn) AuthorizedSubjectMiddleware() func(http.Handler) http.Handler {
	return a.SubjectMiddleware(true)
}
