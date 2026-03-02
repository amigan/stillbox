package authn

import (
	"context"
	_ "embed"
	"errors"
	"net/http"
	"sync"
	"time"

	"dynatron.me/x/stillbox/internal/acl"
	"dynatron.me/x/stillbox/pkg/authz"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/metrics"
	"dynatron.me/x/stillbox/pkg/users"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
)

// Authn performs API key and user JWT authentication.
type Authn interface {
	HUP(*config.Config)

	// VerifyMiddleware will verify any JWT provided with the request.
	VerifyMiddleware() func(http.Handler) http.Handler

	// AuthorizedSubjectMiddleware requires a JWT be set.
	AuthorizedSubjectMiddleware() func(http.Handler) http.Handler

	// PublicSubjectMiddleware allows a Public subject to be set.
	PublicSubjectMiddleware() func(http.Handler) http.Handler

	// NewAccessToken generates a new access token.
	NewAccessToken(username string) string

	// MultipartAPIKeyMiddleware requires a multipart/form-data API key be set.
	MultipartAPIKeyMiddleware(formKey string) func(http.Handler) http.Handler

	// NewAPIKeyToken generates a JWT for use as an API key with the provided expiry and scopes.
	NewAPIKeyToken(username string, expires *time.Time, keyID uuid.UUID, scopes []string) (string, error)

	// LocalAdminMiddleware is used for local Unix domain socket connections..
	LocalAdminMiddleware() func(http.Handler) http.Handler

	// PrivateRoutes installs auth-specific private routes to the Router.
	PrivateRoutes(r chi.Router)

	// PublicRoutes installs auth-specific public routes to the Router.
	PublicRoutes(r chi.Router)

	// AllowInsecureCookie returns whether the request is for a host where we can allow insecure.
	AllowInsecureCookie(*http.Request) bool

	// ValidatePassword does a time-constant validation of the password and returns the User.
	ValidatePassword(ctx context.Context, ust users.Store, username, password string) (*users.User, error)

	// ChangePassword changes a password. Privileged users can specify another username, and do not need to furnish an oldPassword; otherwise the user is grabbed from the context Subject.
	ChangePassword(ctx context.Context, username, oldPassword *string, newPassword string) error

	// CreateAPIKey creates and stores an API key according to rq.
	CreateAPIKey(ctx context.Context, rq CreateAPIKeyRequest) (*users.APIKey, error)
}

type authn struct {
	sync.RWMutex // protects apiKeyACL
	jwtAuthenticator
	rl        *httprate.RateLimiter
	cfg       config.Auth
	ust       users.Store
	apiKeyACL *acl.IP
	metrics   authnMetrics
}

type authnMetrics struct {
	SuccessfulLoginCount    prometheus.Counter `help:"Count of successful logins"`
	FailedLoginCount        prometheus.Counter `help:"Count of failed logins"`
	TokenRefreshCount       prometheus.Counter `help:"Count of token refreshes"`
	FailedTokenRefreshCount prometheus.Counter `help:"Count of failed token refreshes"`
}

func NewAuthn(cfg config.Auth, m metrics.Metrics, ust users.Store) (*authn, error) {
	a := &authn{
		rl:  httprate.NewRateLimiter(5, 5*time.Minute),
		cfg: cfg,
		ust: ust,
	}

	err := a.initAPIKeyACL(cfg.APIKeyACL)
	if err != nil {
		return nil, err
	}

	a.jwtAuthenticator.Init(cfg)

	m.Register("authn", &a.metrics)
	return a, nil
}

func (a *authn) HUP(cfg *config.Config) {
	a.jwtAuthenticator.Init(cfg.Auth)
	err := a.initAPIKeyACL(cfg.Auth.APIKeyACL)
	if err != nil {
		log.Error().Err(err).Msg("API key ACL config reload")
	}
}

func (a *authn) LocalAdminMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		hfn := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			next.ServeHTTP(w, r.WithContext(entities.CtxWithSubject(ctx, entities.NewLocalAdminSubject())))
		}
		return http.HandlerFunc(hfn)
	}
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

//nolint:staticcheck // These are emitted as HTTP errors and should be capitalized
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
		http.Error(w, ErrInternal.Error(), http.StatusInternalServerError)
	}
}

func (a *authn) PublicRoutes(r chi.Router) {
	r.Post("/api/login", a.routeLogin)
	r.Get("/api/login", a.routeLoginPage)
	r.Get("/api/refresh", a.routeRefresh)
}

func (a *authn) PrivateRoutes(r chi.Router) {
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
