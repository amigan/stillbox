package rest

import (
	"errors"
	"fmt"
	"net/http"

	"dynatron.me/x/stillbox/internal/forms"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/talkgroups"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
	"dynatron.me/x/stillbox/pkg/talkgroups/xport"

	"github.com/go-chi/chi/v5"
)

var (
	ErrMissingTGSys = errors.New("missing talkgroup ID and system ID")
	ErrTGIDMismatch = errors.New("url talkgroup ID and document talkgroup ID mismatch")
	ErrSysMismatch  = errors.New("url system ID and document system ID mismatch")
	ErrNoSuchSystem = tgstore.ErrNoSuchSystem
	ErrBadSystem    = errors.New("invalid system")
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
	r.Put(`/{system:\d+}/`, tga.putTalkgroups)
	r.Put(`/{system:\d+}`, tga.putSystem)

	r.Post(`/{system:\d+}/`, tga.postPaginated)
	r.Post(`/`, tga.postPaginated)

	r.Delete(`/{system:\d+}`, tga.deleteSystem)
	r.Delete(`/{system:\d+}/{id:\d+}`, tga.deleteTalkgroup)

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
		res, err = tgs.SystemTGs(ctx, *p.System)
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

type FilterPagination struct {
	tgstore.Pagination

	Filter *string `json:"filter"`
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

	input := &FilterPagination{}
	err = forms.Unmarshal(r, input, forms.WithTag("json"), forms.WithAcceptBlank(), forms.WithOmitEmpty())
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	res := struct {
		Talkgroups []*talkgroups.Talkgroup `json:"talkgroups"`
		Count      int                     `json:"count"`
	}{}

	opts := []tgstore.Option{
		tgstore.WithPagination(&input.Pagination, DefaultPerPage, &res.Count),
		tgstore.WithFilter(input.Filter),
	}

	switch {
	case p.System != nil:
		res.Talkgroups, err = tgs.SystemTGs(ctx, *p.System, opts...)
	default:
		// get all talkgroups
		res.Talkgroups, err = tgs.TGs(ctx, nil, opts...)
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

	input := database.UpsertTalkgroupParams{}

	err = forms.Unmarshal(r, &input, forms.WithTag("json"), forms.WithAcceptBlank(), forms.WithOmitEmpty())
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	if !id.hasBoth() {
		wErr(w, r, autoError(ErrMissingTGSys))
		return
	}

	if input.TGID != 0 && input.TGID != int32(*id.ID) {
		wErr(w, r, autoError(ErrTGIDMismatch))
		return
	}

	if input.SystemID != 0 && input.SystemID != int32(*id.System) {
		wErr(w, r, autoError(ErrSysMismatch))
		return
	}

	input.SystemID = int32(*id.System)
	input.TGID = int32(*id.ID)

	input.Learned = nil // ignore for this call

	record, err := tgs.UpsertTGs(ctx, *id.System, []database.UpsertTalkgroupParams{input})
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	respond(w, r, record[0])
}

func (tga *talkgroupAPI) deleteTalkgroup(w http.ResponseWriter, r *http.Request) {
	var id tgParams
	err := decodeParams(&id, r)
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	if !id.hasBoth() {
		wErr(w, r, badRequest(ErrMissingTGSys))
		return
	}

	ctx := r.Context()
	tgs := tgstore.FromCtx(ctx)

	err = tgs.DeleteTG(ctx, id.ToID())
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (tga *talkgroupAPI) deleteSystem(w http.ResponseWriter, r *http.Request) {
	var id tgParams
	err := decodeParams(&id, r)
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	if id.System == nil {
		wErr(w, r, badRequest(ErrNoSuchSystem))
		return
	}

	ctx := r.Context()
	tgs := tgstore.FromCtx(ctx)

	err = tgs.DeleteSystem(ctx, *id.System)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
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

func (tga *talkgroupAPI) putSystem(w http.ResponseWriter, r *http.Request) {
	var id tgParams
	err := decodeParams(&id, r)
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	if id.System == nil { // don't think this would ever happen
		wErr(w, r, badRequest(ErrBadSystem))
		return
	}

	ctx := r.Context()
	tgs := tgstore.FromCtx(ctx)

	var sysName string

	err = forms.Unmarshal(r, &sysName, forms.WithTag("json"), forms.WithAcceptBlank())
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	err = tgs.CreateSystem(ctx, *id.System, sysName)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
