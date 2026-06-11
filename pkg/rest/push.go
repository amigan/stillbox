package rest

import (
	"net/http"
	"strings"

	"dynatron.me/x/stillbox/internal/forms"
	"dynatron.me/x/stillbox/pkg/notify/push"
	"dynatron.me/x/stillbox/pkg/talkgroups"
	"github.com/go-chi/chi/v5"
)

type pushAPI struct {
	push push.PushNotifier
}

func (pa *pushAPI) Subrouter() http.Handler {
	r := chi.NewRouter()

	r.Get("/vapid", pa.getVAPIDPubkey)
	r.Post("/subscribe", pa.subscribeWebPush)
	r.Post("/subscribe/{client}", pa.subscribeWebPush)
	r.Get("/subscriptions", pa.getSubscriptions)
	r.Delete("/subscriptions", pa.unsubscribe(false))
	r.Post("/subscriptions", pa.subscribe(false))
	r.Put("/subscriptions/{system:[0-9]+}", pa.subscribe(true))
	r.Put("/subscriptions/{system:[0-9]+}/{id:[0-9]+}", pa.subscribe(true))
	r.Delete("/subscriptions/{system:[0-9]+}", pa.unsubscribe(true))
	r.Delete("/subscriptions/{system:[0-9]+}/{id:[0-9]+}", pa.unsubscribe(true))

	return r
}

func (pa *pushAPI) subsetFromParams(r *http.Request) (subSet push.SubscriptionSet, err error) {
	var par tgParams
	err = decodeParams(&par, r)
	if err != nil {
		return
	}

	if par.System != nil {
		if par.ID != nil {
			subSet.Talkgroups = talkgroups.IDs{
				talkgroups.ID{
					System:    uint32(*par.System),
					Talkgroup: uint32(*par.ID),
				},
			}
		} else {
			subSet.Systems = []int32{int32(*par.System)}
		}
	} else { // can't happen
		return subSet, ErrMissingTGSys
	}

	return subSet, nil
}

func (pa *pushAPI) subscribe(singleton bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		var err error
		var subSet push.SubscriptionSet

		switch singleton {
		case true:
			subSet, err = pa.subsetFromParams(r)
			if err != nil {
				wErr(w, r, autoError(err))
				return
			}
		case false:
			err := forms.Unmarshal(r, &subSet, forms.WithTag("json"), forms.WithAcceptBlank(), forms.WithOmitEmpty())
			if err != nil {
				wErr(w, r, badRequestErrText(err))
				return
			}
		}

		err = pa.push.Subscribe(ctx, &subSet)
		if err != nil {
			wErr(w, r, autoError(err))
			return
		}

		w.WriteHeader(http.StatusNoContent)

	}
}

func (pa *pushAPI) unsubscribe(singleton bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		var subSet push.SubscriptionSet
		var err error
		var isEmpty bool

		switch singleton {
		case true:
			subSet, err = pa.subsetFromParams(r)
			if err != nil {
				wErr(w, r, autoError(err))
				return
			}
		case false:
			// if it's empty, delete all subscriptions.
			err := forms.Unmarshal(r, &subSet, forms.WithTag("json"), forms.WithAcceptBlank(), forms.WithOmitEmpty(), forms.WithAcceptEmptyBodyResult(&isEmpty))
			if err != nil {
				wErr(w, r, badRequestErrText(err))
				return
			}
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

	param := struct {
		Client string `param:"client"`
	}{}

	err := decodeParams(&param, r)
	if err != nil {
		return
	}

	var client *string
	if param.Client != "" {
		client = &param.Client
	}

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

	err = pa.push.WebPushSubscribe(ctx, client, sub)
	if err != nil {
		wErr(w, r, autoError(err))
		return
	}

	w.WriteHeader(http.StatusOK)
}

func newPushAPI(pn push.PushNotifier) *pushAPI {
	return &pushAPI{pn}
}
