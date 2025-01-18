package rest

import (
	"errors"
	"net/http"
	"net/url"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/rbac"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/go-viper/mapstructure/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"
)

type API interface {
	Subrouter() http.Handler
}

type api struct {
	baseURL url.URL
}

func New(baseURL url.URL) *api {
	s := &api{baseURL}

	return s
}

func (a *api) Subrouter() http.Handler {
	r := chi.NewMux()

	r.Mount("/talkgroup", new(talkgroupAPI).Subrouter())
	r.Mount("/call", new(callsAPI).Subrouter())
	r.Mount("/user", new(usersAPI).Subrouter())
	r.Mount("/incident", newIncidentsAPI(&a.baseURL).Subrouter())
	r.Mount("/share", newShareHandler(&a.baseURL).Subrouter())

	return r
}

type errResponse struct {
	Err   error  `json:"-"`
	Code  int    `json:"-"`
	Error string `json:"error"`
}

func (e *errResponse) Render(w http.ResponseWriter, r *http.Request) error {
	switch e.Code {
	default:
		log.Error().Str("path", r.URL.Path).Err(e.Err).Int("code", e.Code).Str("msg", e.Error).Msg("request failed")
	}

	render.Status(r, e.Code)

	return nil
}

func badRequest(err error) render.Renderer {
	return &errResponse{
		Err:   err,
		Code:  http.StatusBadRequest,
		Error: "Bad request",
	}
}

func badRequestErrText(err error) render.Renderer {
	return &errResponse{
		Err:   err,
		Code:  http.StatusBadRequest,
		Error: "Bad request: " + err.Error(),
	}
}

func unauthErrText(err error) render.Renderer {
	return &errResponse{
		Err:   err,
		Code:  http.StatusUnauthorized,
		Error: "Unauthorized: " + err.Error(),
	}
}

func forbiddenErrText(err error) render.Renderer {
	return &errResponse{
		Err:   err,
		Code:  http.StatusForbidden,
		Error: "Forbidden: " + err.Error(),
	}
}

func constraintErrText(err error) render.Renderer {
	return &errResponse{
		Err:   err,
		Code:  http.StatusConflict,
		Error: "Constraint violation: " + err.Error(),
	}
}

func recordNotFound(err error) render.Renderer {
	return &errResponse{
		Err:   err,
		Code:  http.StatusNotFound,
		Error: "Record not found",
	}
}

func notFoundErrText(err error) render.Renderer {
	return &errResponse{
		Err:   err,
		Code:  http.StatusNotFound,
		Error: "Record not found: " + err.Error(),
	}
}

func internalError(err error) render.Renderer {
	return &errResponse{
		Err:   err,
		Code:  http.StatusInternalServerError,
		Error: "Internal server error",
	}
}

type errResponder func(error) render.Renderer

var statusMapping = map[error]errResponder{
	tgstore.ErrNoSuchSystem:   notFoundErrText,
	tgstore.ErrNotFound:       notFoundErrText,
	tgstore.ErrInvalidOrderBy: badRequestErrText,
	tgstore.ErrBadDirection:   badRequestErrText,
	tgstore.ErrBadOrder:       badRequestErrText,
	pgx.ErrNoRows:             recordNotFound,
	ErrMissingTGSys:           badRequestErrText,
	ErrTGIDMismatch:           badRequestErrText,
	ErrSysMismatch:            badRequestErrText,
	tgstore.ErrReference:      constraintErrText,
	rbac.ErrBadSubject:        unauthErrText,
	ErrBadAppName:             unauthErrText,
	common.ErrPageOutOfRange:  badRequestErrText,
	rbac.ErrNotAuthorized:     unauthErrText,
}

func autoError(err error) render.Renderer {
	c, ok := statusMapping[err]
	if ok {
		c(err)
	}

	for e, c := range statusMapping { // check if err wraps an error we know about
		if errors.Is(err, e) {
			return c(err)
		}
	}

	if rbac.ErrAccessDenied(err) != nil {
		return forbiddenErrText(err)
	}

	return internalError(err)
}

func wErr(w http.ResponseWriter, r *http.Request, v render.Renderer) {
	err := render.Render(w, r, v)
	if err != nil {
		log.Error().Err(err).Msg("wErr render error")
	}
}

func decodeParams(d interface{}, r *http.Request) error {
	params := chi.RouteContext(r.Context()).URLParams
	m := make(map[string]string, len(params.Keys))

	for i, k := range params.Keys {
		m[k] = params.Values[i]
	}

	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Metadata:         nil,
		Result:           d,
		TagName:          "param",
		WeaklyTypedInput: true,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.TextUnmarshallerHookFunc(),
		),
	})
	if err != nil {
		return err
	}

	return dec.Decode(m)
}

// idOnlyParam checks for a sole URL parameter, id, and writes an errorif this fails.
func idOnlyParam(w http.ResponseWriter, r *http.Request) (uuid.UUID, error) {
	params := struct {
		ID uuid.UUID `param:"id"`
	}{}

	err := decodeParams(&params, r)
	if err != nil {
		wErr(w, r, badRequest(err))
		return uuid.UUID{}, err
	}

	return params.ID, nil
}

func respond(w http.ResponseWriter, r *http.Request, v interface{}) {
	render.DefaultResponder(w, r, v)
}
