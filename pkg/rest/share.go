package rest

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/forms"
	"dynatron.me/x/stillbox/pkg/authz/entities"
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
	ShareRequestCallInfo    ShareRequestType = "callinfo"
	ShareRequestCallDL      ShareRequestType = "callDL"
	ShareRequestIncident    ShareRequestType = "incident"
	ShareRequestIncidentM3U ShareRequestType = "m3u"
	ShareRequestTalkgroups  ShareRequestType = "talkgroups"
)

// shareHandlers returns a ShareHandlers map from the api.
func (s *api) shareHandlers() ShareHandlers {
	return ShareHandlers{
		ShareRequestCall:        s.calls.shareCallRoute,
		ShareRequestCallInfo:    s.respondShareHandler(s.calls.getCallInfo),
		ShareRequestCallDL:      s.calls.shareCallDLRoute,
		ShareRequestIncident:    s.respondShareHandler(s.incidents.getIncident),
		ShareRequestIncidentM3U: s.incidents.getCallsM3U,
		ShareRequestTalkgroups:  s.tgs.getTGsShareRoute,
	}
}

func (rt ShareRequestType) IsValid() bool {
	switch rt {
	case ShareRequestCall, ShareRequestCallInfo, ShareRequestCallDL, ShareRequestIncident,
		ShareRequestIncidentM3U, ShareRequestTalkgroups:
		return true
	}

	return false
}

type ID interface {
}

type ShareHandlerFunc func(id ID, w http.ResponseWriter, r *http.Request)
type ShareHandlers map[ShareRequestType]ShareHandlerFunc
type shareAPI struct {
	baseURL *url.URL
	shnd    ShareHandlers
}

type EntityFunc func(ctx context.Context, id ID) (SharedItem, error)
type SharedItem interface {
	SetShareURL(baseURL url.URL, shareID string)
}

type shareResponse struct {
	ID         ID                `json:"id"`
	Type       shares.EntityType `json:"type"`
	SharedItem SharedItem        `json:"sharedItem,omitempty"`
}

func ShareFrom(ctx context.Context) *shares.Share {
	if share, hasShare := entities.SubjectFrom(ctx).(*shares.Share); hasShare {
		return share
	}

	return nil
}

func (s *api) respondShareHandler(ie EntityFunc) ShareHandlerFunc {
	return func(id ID, w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		share := ShareFrom(ctx)
		if share == nil {
			wErr(w, r, autoError(ErrBadShare))
			return
		}

		res, err := ie(r.Context(), id)
		if err != nil {
			wErr(w, r, autoError(err))
			return
		}

		sRes := shareResponse{
			ID:         share.ID,
			Type:       share.Type,
			SharedItem: res,
		}

		sRes.SharedItem.SetShareURL(*s.baseURL, share.ID)

		respond(w, r, sRes)
	}
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
	r.Post(`/`, sa.listShares)

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

func (sa *shareAPI) listShares(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	shs := shares.FromCtx(ctx)

	p := shares.SharesParams{}
	err := forms.Unmarshal(r, &p, forms.WithTag("json"), forms.WithAcceptBlank(), forms.WithOmitEmpty())
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	shRes, count, err := shs.Shares(ctx, p)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	response := struct {
		Shares     []*shares.Share `json:"shares"`
		TotalCount int             `json:"totalCount"`
	}{
		Shares:     shRes,
		TotalCount: count,
	}

	respond(w, r, &response)
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
			rType = ShareRequestCallInfo
			params.SubID = common.PtrTo(sh.EntityID.String())
		case shares.EntityIncident:
			rType = ShareRequestIncident
		}
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
		sa.shnd[rType](nil, w, r)
	case ShareRequestCall, ShareRequestCallInfo, ShareRequestCallDL:
		var subIDU uuid.UUID
		if params.SubID != nil {
			subIDU, err = uuid.Parse(*params.SubID)
			if err != nil {
				wErr(w, r, badRequest(err))
				return
			}
		} else {
			subIDU = sh.EntityID
		}

		sa.shnd[rType](subIDU, w, r)
	case ShareRequestIncident, ShareRequestIncidentM3U:
		sa.shnd[rType](sh.EntityID, w, r)
	}
}

func (sa *shareAPI) deleteShare(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	shs := shares.FromCtx(ctx)

	p := struct {
		ID string `param:"id"`
	}{}

	err := decodeParams(&p, r)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	err = shs.Delete(ctx, p.ID)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
