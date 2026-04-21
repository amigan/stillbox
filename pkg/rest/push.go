package rest

import (
	"net/http"
	"strings"

	"dynatron.me/x/stillbox/internal/forms"
	"dynatron.me/x/stillbox/pkg/notify/push"
	"github.com/go-chi/chi/v5"
)

type pushAPI struct {
	push push.PushNotifier
}

func (pa *pushAPI) Subrouter() http.Handler {
	r := chi.NewRouter()

	r.Get("/vapid", pa.getVAPIDPubkey)
	r.Post("/subscribe", pa.subscribeWebPush)
	r.Get("/subscriptions", pa.getSubscriptions)
	r.Delete("/subscriptions", pa.unsubscribe)
	r.Post("/subscriptions", pa.addSubscribe)

	return r
}

func (pa *pushAPI) addSubscribe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var subSet push.SubscriptionSet
	err := forms.Unmarshal(r, &subSet, forms.WithTag("json"), forms.WithAcceptBlank(), forms.WithOmitEmpty())
	if err != nil {
		wErr(w, r, badRequestErrText(err))
		return
	}

	err = pa.push.Subscribe(ctx, &subSet)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	w.WriteHeader(http.StatusNoContent)

}

func (pa *pushAPI) unsubscribe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var subSet push.SubscriptionSet
	var isEmpty bool
	err := forms.Unmarshal(r, &subSet, forms.WithTag("json"), forms.WithAcceptBlank(), forms.WithOmitEmpty(), forms.WithAcceptEmptyBodyResult(&isEmpty))
	if err != nil {
		wErr(w, r, badRequestErrText(err))
		return
	}

	if isEmpty {
		subSet.UnsubscribeAll = &isEmpty // it's already true
	}

	err = pa.push.Unsubscribe(ctx, &subSet)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (pa *pushAPI) getSubscriptions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	subs, err := pa.push.Subscriptions(ctx)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	respond(w, r, subs)
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

	sub, err := push.ReadWebPushSubscription(r.Body)
	if err != nil {
		wErr(w, r, badRequestErrText(err))
		return
	}

	err = pa.push.WebPushSubscribe(ctx, sub)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	w.WriteHeader(http.StatusOK)
}

func newPushAPI(pn push.PushNotifier) *pushAPI {
	return &pushAPI{pn}
}
