package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"dynatron.me/x/stillbox/pkg/talkgroups"

	"github.com/go-chi/chi/v5"
	"github.com/go-viper/mapstructure/v2"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"
)

type API interface {
	Subrouter() http.Handler
}

type api struct {
	tgs talkgroups.Store
}

func New(tgs talkgroups.Store) API {
	s := &api{
		tgs: tgs,
	}

	return s
}

func (a *api) Subrouter() http.Handler {
	r := chi.NewMux()

	r.Get("/talkgroup/{system:\\d+}/{id:\\d+}", a.talkgroup)
	r.Get("/talkgroup/{system:\\d+}/", a.talkgroup)
	r.Get("/talkgroup/", a.talkgroup)
	return r
}

var statusMapping = map[error]int{
	talkgroups.ErrNotFound: http.StatusNotFound,
	pgx.ErrNoRows:          http.StatusNotFound,
}

func httpCode(err error) int {
	c, ok := statusMapping[err]
	if ok {
		return c
	}

	for e, c := range statusMapping { // check if err wraps an error we know about
		if errors.Is(err, e) {
			return c
		}
	}

	return http.StatusInternalServerError
}

func (a *api) writeResponse(w http.ResponseWriter, r *http.Request, data interface{}, err error) {
	if err != nil {
		log.Error().Str("path", r.URL.Path).Err(err).Msg("request failed")
		http.Error(w, err.Error(), httpCode(err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	err = enc.Encode(data)
	if err != nil {
		log.Error().Str("path", r.URL.Path).Err(err).Msg("response marshal failed")
		http.Error(w, err.Error(), httpCode(err))
		return
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
	})
	if err != nil {
		return err
	}

	return dec.Decode(m)
}

func (a *api) badReq(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusBadRequest)
}

func (a *api) talkgroup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p := struct {
		System *int `param:"system"`
		ID     *int `param:"id"`
	}{}

	err := decodeParams(&p, r)
	if err != nil {
		a.badReq(w, err)
		return
	}

	var res interface{}
	switch {
	case p.System != nil && p.ID != nil:
		res, err = a.tgs.TG(ctx, talkgroups.TG(*p.System, *p.ID))
	case p.System != nil:
		res, err = a.tgs.SystemTGs(ctx, int32(*p.System))
	default:
		res, err = a.tgs.TGs(ctx, nil)
	}

	a.writeResponse(w, r, res, err)
}
