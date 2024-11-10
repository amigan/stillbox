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
}

func New() API {
	s := new(api)

	return s
}

func (a *api) Subrouter() http.Handler {
	r := chi.NewMux()

	r.Mount("/talkgroup", new(talkgroupAPI).routes())

	return r
}

type errResponse struct {
	text string
	code int
}

var statusMapping = map[error]errResponse{
	talkgroups.ErrNotFound: {talkgroups.ErrNotFound.Error(), http.StatusNotFound},
	pgx.ErrNoRows:          {"no such record", http.StatusNotFound},
}

func httpCode(err error) (string, int) {
	c, ok := statusMapping[err]
	if ok {
		return c.text, c.code
	}

	for e, c := range statusMapping { // check if err wraps an error we know about
		if errors.Is(err, e) {
			return c.text, c.code
		}
	}

	return err.Error(), http.StatusInternalServerError
}

func writeResponse(w http.ResponseWriter, r *http.Request, data interface{}, err error) {
	if err != nil {
		log.Error().Str("path", r.URL.Path).Err(err).Msg("request failed")
		text, code := httpCode(err)
		http.Error(w, text, code)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	err = enc.Encode(data)
	if err != nil {
		log.Error().Str("path", r.URL.Path).Err(err).Msg("response marshal failed")
		text, code := httpCode(err)
		http.Error(w, text, code)
		return
	}
}

func reqErr(w http.ResponseWriter, err error, code int) {
	http.Error(w, err.Error(), code)
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

func badReq(w http.ResponseWriter, err error) {
	reqErr(w, err, http.StatusBadRequest)
}
