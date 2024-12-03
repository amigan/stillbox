package rest

import (
	"fmt"
	"net/http"

	"dynatron.me/x/stillbox/internal/forms"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/talkgroups"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
	"dynatron.me/x/stillbox/pkg/talkgroups/xport"

	"github.com/go-chi/chi/v5"
)

const DefaultPerPage = 20

type talkgroupAPI struct {
}

func (tga *talkgroupAPI) Subrouter() http.Handler {
	r := chi.NewMux()

	r.Get(`/{system:\d+}/{id:\d+}`, tga.get)
	r.Get(`/{system:\d+}/`, tga.get)
	r.Get("/", tga.get)

	r.Put(`/{system:\d+}/{id:\d+}`, tga.put)
	r.Put(`/{system:\d+}`, tga.putTalkgroups)

	r.Post(`/{system:\d+}/`, tga.postPaginated)
	r.Post(`/`, tga.postPaginated)

	r.Post("/import", tga.tgImport)

	r.Post("/export", tga.tgExport)

	return r
}

type tgParams struct {
	System *int `param:"system"`
	ID     *int `param:"id"`
}

func (t tgParams) hasBoth() bool {
	return t.System != nil && t.ID != nil
}

func (t tgParams) ToID() talkgroups.ID {
	nilOr := func(i *int) uint32 {
		if i == nil {
			return 0
		}

		return uint32(*i)
	}

	return talkgroups.ID{
		System:    nilOr(t.System),
		Talkgroup: nilOr(t.ID),
	}
}

func (tga *talkgroupAPI) get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tgs := tgstore.FromCtx(ctx)

	var p tgParams

	err := decodeParams(&p, r)
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	var res interface{}
	switch {
	case p.hasBoth():
		res, err = tgs.TG(ctx, talkgroups.TG(*p.System, *p.ID))
	case p.System != nil:
		res, err = tgs.SystemTGs(ctx, int32(*p.System))
	default:
		// get all talkgroups
		res, err = tgs.TGs(ctx, nil)
	}

	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	respond(w, r, res)
}

func (tga *talkgroupAPI) postPaginated(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tgs := tgstore.FromCtx(ctx)

	var p tgParams

	err := decodeParams(&p, r)
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	input := &tgstore.Pagination{}
	err = forms.Unmarshal(r, input, forms.WithTag("json"), forms.WithAcceptBlank(), forms.WithOmitEmpty())
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	res := struct {
		Talkgroups []*talkgroups.Talkgroup `json:"talkgroups"`
		Count      int                     `json:"count"`
	}{}
	switch {
	case p.System != nil:
		res.Talkgroups, err = tgs.SystemTGs(ctx, int32(*p.System), tgstore.WithPagination(input, DefaultPerPage, &res.Count))
	default:
		// get all talkgroups
		res.Talkgroups, err = tgs.TGs(ctx, nil, tgstore.WithPagination(input, DefaultPerPage, &res.Count))
	}

	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	respond(w, r, res)
}

func (tga *talkgroupAPI) put(w http.ResponseWriter, r *http.Request) {
	var id tgParams
	err := decodeParams(&id, r)
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	ctx := r.Context()
	tgs := tgstore.FromCtx(ctx)

	input := database.UpdateTalkgroupParams{}

	err = forms.Unmarshal(r, &input, forms.WithTag("json"), forms.WithAcceptBlank(), forms.WithOmitEmpty())
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	input.Learned = nil // ignore for this call

	record, err := tgs.UpdateTG(ctx, input)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	respond(w, r, record)
}

func (tga *talkgroupAPI) tgExport(w http.ResponseWriter, r *http.Request) {
	var expJob xport.ExportJob
	ctx := r.Context()

	err := forms.Unmarshal(r, &expJob, forms.WithAcceptBlank(), forms.WithOmitEmpty())
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	w.Header().Set("Access-Control-Expose-Headers", "Content-Disposition")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=stillbox_%s", expJob.TemplateFileName))
	w.Header().Set("Content-Type", "text/xml")

	err = expJob.Export(ctx, w)
	if err != nil {
		wErr(w, r, autoError(err))
	}
}

func (tga *talkgroupAPI) tgImport(w http.ResponseWriter, r *http.Request) {
	var impJob xport.ImportJob
	err := forms.Unmarshal(r, &impJob, forms.WithTag("json"), forms.WithAcceptBlank(), forms.WithOmitEmpty())
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}
	recs, err := impJob.Import(r.Context())
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	respond(w, r, recs)
}

func (tga *talkgroupAPI) putTalkgroups(w http.ResponseWriter, r *http.Request) {
	var id tgParams
	err := decodeParams(&id, r)
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	if id.System == nil { // don't think this would ever happen
		wErr(w, r, badRequest(tgstore.ErrNoSuchSystem))
		return
	}

	ctx := r.Context()
	tgs := tgstore.FromCtx(ctx)

	var input []database.UpsertTalkgroupParams

	err = forms.Unmarshal(r, &input, forms.WithTag("json"), forms.WithAcceptBlank(), forms.WithOmitEmpty())
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	record, err := tgs.UpsertTGs(ctx, *id.System, input)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	respond(w, r, record)
}
