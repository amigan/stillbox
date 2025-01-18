package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/forms"
	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/incidents"
	"dynatron.me/x/stillbox/pkg/incidents/incstore"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type shareAPI struct {
	baseURL *url.URL
}

func newShareHandler(baseURL *url.URL) API {
	return &shareAPI{baseURL}
}

func (ia *shareAPI) Subrouter() http.Handler {
	r := chi.NewMux()

	//r.Get(`/{id:[A-Za-z0-9_-]{20,}}`, ia.getShare)
	//r.Post('/create', ia.createShare)
	//r.Delete(`/{id:[A-Za-z0-9_-]{20,}}`, ia.deleteShare)
	//r.Get(`/`, ia.getShares)

	return r
}

func (ia *shareAPI) listIncidents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	incs := incstore.FromCtx(ctx)

	p := incstore.IncidentsParams{}
	err := forms.Unmarshal(r, &p, forms.WithTag("json"), forms.WithAcceptBlank(), forms.WithOmitEmpty())
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	res := struct {
		Incidents []incstore.Incident `json:"incidents"`
		Count     int                 `json:"count"`
	}{}

	res.Incidents, res.Count, err = incs.Incidents(ctx, p)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	respond(w, r, res)
}

func (ia *shareAPI) createIncident(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	incs := incstore.FromCtx(ctx)

	p := incidents.Incident{}
	err := forms.Unmarshal(r, &p, forms.WithTag("json"), forms.WithAcceptBlank(), forms.WithOmitEmpty())
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	inc, err := incs.CreateIncident(ctx, p)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	respond(w, r, inc)
}

func (ia *shareAPI) getIncident(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	incs := incstore.FromCtx(ctx)

	id, err := idOnlyParam(w, r)
	if err != nil {
		return
	}

	inc, err := incs.Incident(ctx, id)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	respond(w, r, inc)
}

func (ia *shareAPI) updateIncident(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	incs := incstore.FromCtx(ctx)

	id, err := idOnlyParam(w, r)
	if err != nil {
		return
	}

	p := incstore.UpdateIncidentParams{}
	err = forms.Unmarshal(r, &p, forms.WithTag("json"), forms.WithAcceptBlank(), forms.WithOmitEmpty())
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	inc, err := incs.UpdateIncident(ctx, id, p)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	respond(w, r, inc)
}

func (ia *shareAPI) deleteIncident(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	incs := incstore.FromCtx(ctx)

	urlParams := struct {
		ID uuid.UUID `param:"id"`
	}{}

	err := decodeParams(&urlParams, r)
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	err = incs.DeleteIncident(ctx, urlParams.ID)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type CallIncidentParams2 struct {
	Add   jsontypes.UUIDs `json:"add"`
	Notes json.RawMessage `json:"notes"`

	Remove jsontypes.UUIDs `json:"remove"`
}

func (ia *shareAPI) postCalls(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	incs := incstore.FromCtx(ctx)

	id, err := idOnlyParam(w, r)
	if err != nil {
		return
	}

	p := CallIncidentParams2{}
	err = forms.Unmarshal(r, &p, forms.WithTag("json"), forms.WithAcceptBlank(), forms.WithOmitEmpty())
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	err = incs.AddRemoveIncidentCalls(ctx, id, p.Add.UUIDs(), p.Notes, p.Remove.UUIDs())
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (ia *shareAPI) getCallsM3U(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	incs := incstore.FromCtx(ctx)
	tgst := tgstore.FromCtx(ctx)

	id, err := idOnlyParam(w, r)
	if err != nil {
		return
	}

	inc, err := incs.Incident(ctx, id)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	b := new(bytes.Buffer)

	callUrl := common.PtrTo(*ia.baseURL)

	b.WriteString("#EXTM3U\n\n")
	for _, c := range inc.Calls {
		tg, err := tgst.TG(ctx, c.TalkgroupTuple())
		if err != nil {
			wErr(w, r, autoError(err))
			return
		}
		var from string
		if c.Source != 0 {
			from = fmt.Sprintf(" from %d", c.Source)
		}

		callUrl.Path = "/api/call/" + c.ID.String()

		fmt.Fprintf(b, "#EXTINF:%d,%s%s (%s)\n%s\n\n",
			c.Duration.Seconds(),
			tg.StringTag(true),
			from,
			c.DateTime.Format("15:04 01/02"),
			callUrl,
		)
	}

	// Not a lot of agreement on which MIME type to use for non-HLS m3u,
	// let's hope this is good enough
	w.Header().Set("Content-Type", "audio/x-mpegurl")
	w.WriteHeader(http.StatusOK)
	_, _ = b.WriteTo(w)
}
