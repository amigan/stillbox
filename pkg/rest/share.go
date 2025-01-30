package rest

import (
	"errors"
	"net/http"
	"net/url"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/forms"
	"dynatron.me/x/stillbox/pkg/rbac/entities"
	"dynatron.me/x/stillbox/pkg/shares"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

var (
	ErrBadShare = errors.New("bad share request type")
)

type ShareRequestType string

const (
	ShareRequestCall        ShareRequestType = "call"
	ShareRequestCallDL      ShareRequestType = "callDL"
	ShareRequestIncident    ShareRequestType = "incident"
	ShareRequestIncidentM3U ShareRequestType = "m3u"
	ShareRequestTalkgroups  ShareRequestType = "talkgroups"
)

func (rt ShareRequestType) IsValid() bool {
	switch rt {
	case ShareRequestCall, ShareRequestCallDL, ShareRequestIncident,
		ShareRequestIncidentM3U, ShareRequestTalkgroups:
		return true
	}

	return false
}

func (rt ShareRequestType) IsValidSubtype() bool {
	switch rt {
	case ShareRequestCall, ShareRequestCallDL, ShareRequestTalkgroups:
		return true
	}

	return false
}

type ID interface {
}

type HandlerFunc func(id ID, share *shares.Share, w http.ResponseWriter, r *http.Request)
type ShareHandlers map[ShareRequestType]HandlerFunc
type shareAPI struct {
	baseURL *url.URL
	shnd    ShareHandlers
}

func newShareAPI(baseURL *url.URL, shnd ShareHandlers) *shareAPI {
	return &shareAPI{
		baseURL: baseURL,
		shnd:    shnd,
	}
}

func (sa *shareAPI) Subrouter() http.Handler {
	r := chi.NewMux()

	r.Post(`/create`, sa.createShare)
	r.Delete(`/{id:[A-Za-z0-9_-]{20,}}`, sa.deleteShare)

	return r
}

func (sa *shareAPI) RootRouter() http.Handler {
	r := chi.NewMux()

	r.Get("/{shareId:[A-Za-z0-9_-]{20,}}", sa.routeShare)
	r.Get("/{shareId:[A-Za-z0-9_-]{20,}}/{type}", sa.routeShare)
	r.Get("/{shareId:[A-Za-z0-9_-]{20,}}.{type}", sa.routeShare)
	r.Get("/{shareId:[A-Za-z0-9_-]{20,}}/{type}/{subID}", sa.routeShare)
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

func (sa *shareAPI) routeShare(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	shs := shares.FromCtx(ctx)

	params := struct {
		ID    string  `param:"shareId"`
		Type  *string `param:"type"`
		SubID *string `param:"subID"`
	}{}

	err := decodeParams(&params, r)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	id := params.ID
	sh, err := shs.GetShare(ctx, id)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	var rType ShareRequestType

	if params.Type != nil {
		rType = ShareRequestType(*params.Type)
	} else {
		switch sh.Type {
		case shares.EntityCall:
			rType = ShareRequestCall
			params.SubID = common.PtrTo(sh.EntityID.String())
		case shares.EntityIncident:
			rType = ShareRequestIncident
		}
		w.Header().Set("X-Share-Type", string(rType))
	}

	if !rType.IsValid() {
		wErr(w, r, autoError(ErrBadShare))
		return
	}

	if sh.Expiration != nil && sh.Expiration.Time().Before(time.Now()) {
		wErr(w, r, autoError(shares.ErrNoShare))
		return
	}

	ctx = entities.CtxWithSubject(ctx, sh)
	r = r.WithContext(ctx)

	switch rType {
	case ShareRequestTalkgroups:
		sa.shnd[rType](nil, sh, w, r)
	case ShareRequestCall, ShareRequestCallDL:
		if params.SubID == nil {
			wErr(w, r, autoError(ErrBadShare))
			return
		}

		subIDU, err := uuid.Parse(*params.SubID)
		if err != nil {
			wErr(w, r, badRequest(err))
			return
		}
		sa.shnd[rType](subIDU, sh, w, r)
	case ShareRequestIncident, ShareRequestIncidentM3U:
		sa.shnd[rType](sh.EntityID, sh, w, r)
	}
}

func (sa *shareAPI) deleteShare(w http.ResponseWriter, r *http.Request) {
}
