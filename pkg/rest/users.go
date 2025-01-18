package rest

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"dynatron.me/x/stillbox/pkg/auth"
	"dynatron.me/x/stillbox/pkg/rbac"
	"dynatron.me/x/stillbox/pkg/users"

	"github.com/go-chi/chi/v5"
)

var (
	ErrBadAppName = errors.New("bad app name")
)

type usersAPI struct {
}

func (ua *usersAPI) Subrouter() http.Handler {
	r := chi.NewMux()

	r.Get(`/prefs/{appName}`, ua.getPrefs)
	r.Put(`/prefs/{appName}`, ua.putPrefs)

	return r
}

func (ua *usersAPI) getPrefs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	username := auth.UsernameFrom(ctx)

	if username == nil {
		wErr(w, r, autoError(rbac.ErrBadSubject))
		return
	}

	p := struct {
		AppName *string `param:"appName"`
	}{}

	err := decodeParams(&p, r)
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	if p.AppName == nil {
		wErr(w, r, autoError(ErrBadAppName))
		return
	}

	us := users.FromCtx(ctx)
	prefs, err := us.UserPrefs(ctx, *username, *p.AppName)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	_, _ = w.Write(prefs)
}

func (ua *usersAPI) putPrefs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	username := auth.UsernameFrom(ctx)

	if username == nil {
		wErr(w, r, autoError(rbac.ErrBadSubject))
		return
	}

	contentType := strings.Split(r.Header.Get("Content-Type"), ";")[0]
	if contentType != "application/json" {
		wErr(w, r, badRequest(errors.New("only json accepted")))
		return
	}

	p := struct {
		AppName *string `param:"appName"`
	}{}

	err := decodeParams(&p, r)
	if err != nil {
		wErr(w, r, badRequest(err))
		return
	}

	if p.AppName == nil {
		wErr(w, r, autoError(ErrBadAppName))
		return
	}

	prefs, err := io.ReadAll(r.Body)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	us := users.FromCtx(ctx)
	err = us.SetUserPrefs(ctx, *username, *p.AppName, prefs)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	_, _ = w.Write(prefs)
}
