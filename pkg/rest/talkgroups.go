package rest

import (
	"net/http"

	"dynatron.me/x/stillbox/internal/forms"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/talkgroups"

	"github.com/go-chi/chi/v5"
)

type talkgroupAPI struct {
}

func (tga *talkgroupAPI) Subrouter() http.Handler {
	r := chi.NewMux()

	r.Get("/{system:\\d+}/{id:\\d+}", tga.get)
	r.Put("/{system:\\d+}/{id:\\d+}", tga.put)
	r.Get("/{system:\\d+}/", tga.get)
	r.Get("/", tga.get)
	r.Put("/import", tga.tgImport)

	return r
}

type tgParams struct {
	System *int `param:"system"`
	ID     *int `param:"id"`
}

func (t tgParams) haveBoth() bool {
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
	tgs := talkgroups.StoreFrom(ctx)

	var p tgParams

	err := decodeParams(&p, r)
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	var res interface{}
	switch {
	case p.System != nil && p.ID != nil:
		res, err = tgs.TG(ctx, talkgroups.TG(*p.System, *p.ID))
	case p.System != nil:
		res, err = tgs.SystemTGs(ctx, int32(*p.System))
	default:
		res, err = tgs.TGs(ctx, nil)
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
	tgs := talkgroups.StoreFrom(ctx)

	input := database.UpdateTalkgroupParams{}

	err = forms.Unmarshal(r, &input, forms.WithTag("json"), forms.WithAcceptBlank(), forms.WithOmitEmpty())
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}
	input.ID = id.ToID().Pack()

	record, err := tgs.UpdateTG(ctx, input)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	respond(w, r, record)
}

func (tga *talkgroupAPI) tgImport(w http.ResponseWriter, r *http.Request) {
	var impJob talkgroups.ImportJob
	err := forms.Unmarshal(r, &impJob, forms.WithTag("json"), forms.WithAcceptBlank(), forms.WithOmitEmpty())
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}
	recs, err := impJob.Import()
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	respond(w, r, recs)
}
