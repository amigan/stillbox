package rest

import (
	"net/http"
	"strings"

	"dynatron.me/x/stillbox/internal/forms"
	"dynatron.me/x/stillbox/pkg/notify/webpush"
	"github.com/go-chi/chi/v5"
)

type pushAPI struct {
	push webpush.PushNotifier
}

func (pa *pushAPI) Subrouter() http.Handler {
	r := chi.NewRouter()

	r.Get("/vapid", pa.getVAPIDPubkey)
	r.Post("/subscribe", pa.subscribeWebPush)

	return r
}

func (pa *pushAPI) getVAPIDPubkey(w http.ResponseWriter, r *http.Request) {
	respond(w, r, pa.push.VAPIDPublicKey())
}

func (pa *pushAPI) subscribeWebPush(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	contentType := strings.Split(r.Header.Get("Content-Type"), ";")[0]
	switch contentType {
	case "application/json", "":
	default:
		wErr(w, r, badRequest(forms.ErrContentType))
		return
	}

	sub, err := webpush.ReadSubscription(r.Body)
	if err != nil {
		wErr(w, r, badRequestErrText(err))
		return
	}

	err = pa.push.Subscribe(ctx, sub)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	w.WriteHeader(http.StatusOK)
}

func newPushAPI(pn webpush.PushNotifier) *pushAPI {
	return &pushAPI{pn}
}
