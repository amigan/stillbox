package rest

import (
	"bytes"
	"context"
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

type incidentsAPI struct {
	baseURL *url.URL
}

func newIncidentsAPI(baseURL *url.URL) *incidentsAPI {
	return &incidentsAPI{baseURL}
}

func (ia *incidentsAPI) Subrouter() http.Handler {
	r := chi.NewMux()

	r.Get(`/{id:[a-f0-9-]+}`, ia.getIncidentRoute)
	r.Get(`/{id:[a-f0-9-]+}.m3u`, ia.getCallsM3URoute)
	r.Post(`/new`, ia.createIncident)
	r.Post(`/`, ia.listIncidents)
	r.Post(`/{id:[a-f0-9-]+}/calls`, ia.postCalls)

	r.Patch(`/{id:[a-f0-9-]+}`, ia.updateIncident)

	r.Delete(`/{id:[a-f0-9-]+}`, ia.deleteIncident)

	return r
}

func (ia *incidentsAPI) listIncidents(w http.ResponseWriter, r *http.Request) {
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

func (ia *incidentsAPI) createIncident(w http.ResponseWriter, r *http.Request) {
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

func (ia *incidentsAPI) getIncidentRoute(w http.ResponseWriter, r *http.Request) {
	id, err := idOnlyParam(w, r)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	e, err := ia.getIncident(r.Context(), id)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	respond(w, r, e)
}

func (ia *incidentsAPI) getIncident(ctx context.Context, id ID) (SharedItem, error) {
	incs := incstore.FromCtx(ctx)
	return incs.Incident(ctx, id.(uuid.UUID))
}

func (ia *incidentsAPI) updateIncident(w http.ResponseWriter, r *http.Request) {
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

func (ia *incidentsAPI) deleteIncident(w http.ResponseWriter, r *http.Request) {
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

type CallIncidentParams struct {
	Add   jsontypes.UUIDs `json:"add"`
	Notes json.RawMessage `json:"notes"`

	Remove jsontypes.UUIDs `json:"remove"`
}

func (ia *incidentsAPI) postCalls(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	incs := incstore.FromCtx(ctx)

	id, err := idOnlyParam(w, r)
	if err != nil {
		return
	}

	p := CallIncidentParams{}
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

func (ia *incidentsAPI) getCallsM3URoute(w http.ResponseWriter, r *http.Request) {
	id, err := idOnlyParam(w, r)
	if err != nil {
		return
	}

	ia.getCallsM3U(id, w, r)
}

func (ia *incidentsAPI) getCallsM3U(id ID, w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	incs := incstore.FromCtx(ctx)
	tgst := tgstore.FromCtx(ctx)

	inc, err := incs.Incident(ctx, id.(uuid.UUID))
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	b := new(bytes.Buffer)

	callUrl := common.PtrTo(*ia.baseURL)
	urlRoot := "/api/call"
	filename := inc.PlaylistFilename()
	share := ShareFrom(ctx)
	if share != nil {
		urlRoot = fmt.Sprintf("/share/%s/call/", share.ID)
		filename += "_" + share.ID
	}
	filename += ".m3u"

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

		callUrl.Path = urlRoot + c.ID.String()

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
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.WriteHeader(http.StatusOK)
	_, _ = b.WriteTo(w)
}
