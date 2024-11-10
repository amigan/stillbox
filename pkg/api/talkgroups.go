package api

import (
	"fmt"
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

func (tga *talkgroupAPI) talkgroup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tgs := talkgroups.StoreFrom(ctx)

	var p tgParams

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
	var id tgParams
	err := decodeParams(&id, r)
	if err != nil {
		badReq(w, err)
		return
	}
	/*
		ctx := r.Context()
		tgs := talkgroups.StoreFrom(ctx)

		tg, err := tgs.TG(ctx, id.ToID())
		switch err {
		case nil:
		case talkgroups.ErrNotFound:
			reqErr(w, err, http.StatusNotFound)
			return
		default:
			reqErr(w, err, http.StatusInternalServerError)
		}
	*/

	input := struct {
		Name        *string  `form:"name"`
		AlphaTag    *string  `form:"alpha_tag"`
		TgGroup     *string  `form:"tg_group"`
		Frequency   *int32   `form:"frequency"`
		Metadata    []byte   `form:"metadata"`
		Tags        []string `form:"tags"`
		Alert       *bool    `form:"alert"`
		AlertConfig []byte   `form:"alert_config"`
		Weight      *float32 `form:"weight"`
	}{}

	err = forms.Unmarshal(r, &input, forms.WithAcceptBlank(), forms.WithOmitEmpty())
	if err != nil {
		reqErr(w, err, http.StatusBadRequest)
		return
	}
	fmt.Fprintf(w, "%+v\n", input)
}
