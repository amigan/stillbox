package rest

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"dynatron.me/x/stillbox/internal/forms"
	"dynatron.me/x/stillbox/pkg/rbac"
	"dynatron.me/x/stillbox/pkg/shares"

	"github.com/go-chi/chi/v5"
)

var (
	ErrBadShare = errors.New("bad share request type")
)

type ShareRequestType string

const (
	ShareRequestCall        ShareRequestType = "call"
	ShareRequestIncident    ShareRequestType = "incident"
	ShareRequestIncidentM3U ShareRequestType = "m3u"
)

func (rt ShareRequestType) IsValid() bool {
	switch rt {
	case ShareRequestCall, ShareRequestIncident, ShareRequestIncidentM3U:
		return true
	}

	return false
}

type ShareHandlers map[shares.EntityType]http.Handler
type shareAPI struct {
	baseURL *url.URL

	router http.Handler
}

func newShareAPI(baseURL *url.URL, hand http.Handler) publicAPI {
	return &shareAPI{
		baseURL: baseURL,
		router:  hand,
	}
}

func (sa *shareAPI) Subrouter() http.Handler {
	r := chi.NewMux()

	r.Post(`/create`, sa.createShare)
	r.Delete(`/{id:[A-Za-z0-9_-]{20,}}`, sa.deleteShare)

	return r
}

func (sa *shareAPI) PublicRouter() http.Handler {
	r := chi.NewMux()

	r.Get(`/{type}/{id:[A-Za-z0-9_-]{20,}}`, sa.getShare)
	r.Get(`/{type}/{id:[A-Za-z0-9_-]{20,}}*`, sa.getShare)

	return r
}

func (sa *shareAPI) createShare(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	shs := shares.FromCtx(ctx)

	p := shares.CreateShareParams{}

	err := forms.Unmarshal(r, &p, forms.WithTag("json"), forms.WithAcceptBlank(), forms.WithOmitEmpty())
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	sh, err := shs.NewShare(ctx, p)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	respond(w, r, sh)
}

func (sa *shareAPI) deleteShare(w http.ResponseWriter, r *http.Request) {
}

func (sa *shareAPI) getShare(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	shs := shares.FromCtx(ctx)

	rType, id, err := shareParams(w, r)
	if err != nil {
		return
	}

	if !rType.IsValid() {
		wErr(w, r, autoError(ErrBadShare))
	}

	sh, err := shs.GetShare(ctx, id)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	if sh.Expiration != nil && sh.Expiration.Time().Before(time.Now()) {
		wErr(w, r, autoError(shares.ErrNoShare))
		return
	}

	ctx = rbac.CtxWithSubject(ctx, sh)
	r = r.WithContext(ctx)

	switch rType {
	case ShareRequestCall, ShareRequestIncident:
		r.URL.Path = fmt.Sprintf("/%s/%s", rType, sh.EntityID.String())
	case ShareRequestIncidentM3U:
		r.URL.Path = fmt.Sprintf("/incident/%s.m3u", sh.EntityID.String())
	}

	sa.router.ServeHTTP(w, r)
}

// idOnlyParam checks for a sole URL parameter, id, and writes an errorif this fails.
func shareParams(w http.ResponseWriter, r *http.Request) (rType ShareRequestType, rID string, err error) {
	params := struct {
		Type string `param:"type"`
		ID   string `param:"id"`
	}{}

	err = decodeParams(&params, r)
	if err != nil {
		wErr(w, r, badRequest(err))
		return "", "", err
	}

	return ShareRequestType(params.Type), params.ID, nil
}
