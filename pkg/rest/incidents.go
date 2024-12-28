package rest

import (
	"net/http"

	"dynatron.me/x/stillbox/internal/forms"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/incidents"
	"dynatron.me/x/stillbox/pkg/incidents/incstore"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type incidentsAPI struct {
}

func (ia *incidentsAPI) Subrouter() http.Handler {
	r := chi.NewMux()

	r.Get(`/{id:[a-f0-9-]+}`, ia.getIncident)

	r.Post(`/create`, ia.createIncident)
	r.Post(`/`, ia.listIncidents)

	r.Put(`/{id:[a-f0-9]+}`, ia.updateIncident)

	r.Delete(`/{id:[a-f0-9]+}`, ia.deleteIncident)

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
		Incidents []database.Incident `json:"incidents"`
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

func (ia *incidentsAPI) getIncident(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	incs := incstore.FromCtx(ctx)

	params := struct {
		ID uuid.UUID `param:"id"`
	}{}

	err := decodeParams(&params, r)
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	inc, err := incs.Incident(ctx, params.ID)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	respond(w, r, inc)
}

func (ia *incidentsAPI) updateIncident(w http.ResponseWriter, r *http.Request) {
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

	p := incstore.UpdateIncidentParams{}
	err = forms.Unmarshal(r, &p, forms.WithTag("json"), forms.WithAcceptBlank(), forms.WithOmitEmpty())
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	inc, err := incs.UpdateIncident(ctx, urlParams.ID, p)
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
