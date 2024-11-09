package api

import (
	"net/http"

	"dynatron.me/x/stillbox/internal/forms"
	"dynatron.me/x/stillbox/pkg/talkgroups"

	"github.com/go-chi/chi/v5"
)

type talkgroupAPI struct {
}

func (tga *talkgroupAPI) routes() http.Handler {
	r := chi.NewMux()

	r.Get("/{system:\\d+}/{id:\\d+}", tga.talkgroup)
	r.Put("/{system:\\d+}/{id:\\d+}", tga.putTalkgroup)
	r.Get("/{system:\\d+}/", tga.talkgroup)
	r.Get("/", tga.talkgroup)

	return r
}

func (tga *talkgroupAPI) talkgroup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tgs := talkgroups.StoreFrom(ctx)
	p := struct {
		System *int `param:"system"`
		ID     *int `param:"id"`
	}{}

	err := decodeParams(&p, r)
	if err != nil {
		badReq(w, err)
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

	writeResponse(w, r, res, err)
}

func (tga *talkgroupAPI) putTalkgroup(w http.ResponseWriter, r *http.Request) {
}
