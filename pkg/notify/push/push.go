package push

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"

	"dynatron.me/x/stillbox/pkg/alerting/alert"
	"dynatron.me/x/stillbox/pkg/authz"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/notify"
	"dynatron.me/x/stillbox/pkg/settings"
	"dynatron.me/x/stillbox/pkg/users"
	"github.com/rs/zerolog/log"
)

const (
	VAPIDSettingsKey = "vapid.stillbox"
)

type vapidKeys struct {
	Public  string `json:"pubKey"`
	Private string `json:"privKey"`
}

type PushNotifier interface {
	notify.NotifierBackend

	// VAPIDPublicKey returns the VAPID public key of the instance.
	VAPIDPublicKey() string

	// Subscribe stores a user's subscription.
	Subscribe(ctx context.Context, sub *Subscription) error
}

func ReadSubscription(r io.Reader) (*Subscription, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	sub := new(Subscription)
	err = json.Unmarshal(raw, sub)
	if err != nil {
		return nil, err
	}

	sub.raw = raw

	return sub, nil
}

type pushNotifier struct {
	settings settings.Store
	db       database.Store
	keys     vapidKeys
	webPush  Sender
	baseURL  *url.URL
}

type Sender interface {
	Send(ctx context.Context, subs []database.GetSubscriptionsSubscribedRow, al *alert.RenderedAlert) error
}

type pushNotifierOption func(*pushNotifier)

// WithSender configures a non-default Sender for the push notifier.
// This is for testing.
func WithSender(s Sender) pushNotifierOption {
	return func(pn *pushNotifier) {
		pn.webPush = s
	}
}

func (pn *pushNotifier) VAPIDPublicKey() string {
	return pn.keys.Public
}

func (pn *pushNotifier) Dispatch(ctx context.Context, renderedAlerts *alert.RenderedAlertBatch) error {
	// XXX This must be made to use an iterator!
	for _, al := range renderedAlerts.Alerts {
		notifySubs, err := pn.db.GetSubscriptionsSubscribed(ctx, int32(al.TGID.System), int32(al.TGID.Talkgroup))
		if err != nil {
			log.Error().Err(err).Int32("sys", al.Talkgroup.SystemID).Int32("tgid", al.Talkgroup.TGID).Msg("getSubscriptionsSubscribed")
			continue
		}
		err = pn.webPush.Send(ctx, notifySubs, &al)
		if err != nil {
			log.Error().Err(err).Int32("sys", al.Talkgroup.SystemID).Int32("tgid", al.Talkgroup.TGID).Msg("send")
		}
	}
	return nil
}

func (pn *pushNotifier) Subscribe(ctx context.Context, sub *Subscription) error {
	user, err := users.UserCheck(ctx, authz.UseResource(entities.ResourcePushSub), "create")
	if err != nil {
		return err
	}

	return pn.db.CreatePushSubscription(ctx, user.ID.Int(), sub.Expiration, sub.raw)
}

func (pn *pushNotifier) DeleteSubscription(ctx context.Context, sub json.RawMessage) error {
	return pn.db.DeletePushSubscriptionBySub(ctx, sub)
}

func NewPushNotifier(ctx context.Context, baseURL *url.URL, db database.Store, rbacSvc authz.RBAC, setStore settings.Store, opts ...pushNotifierOption) (*pushNotifier, error) {
	ctx = authz.CtxWithRBAC(ctx, rbacSvc)
	ctx = entities.CtxWithServiceSubject(ctx, "pushNotifier")
	pn := &pushNotifier{
		db:       db,
		settings: setStore,
		baseURL:  baseURL,
	}

	for _, opt := range opts {
		opt(pn)
	}

	if pn.webPush == nil {
		pn.webPush = newWebpushSender(pn)
	}

	err := setStore.GetInto(ctx, VAPIDSettingsKey, &pn.keys)
	if err != nil {
		if !errors.Is(err, settings.ErrNoSetting) {
			return nil, err
		} else {
			err := pn.keys.generate()
			if err != nil {
				return nil, err
			}

			err = setStore.Set(ctx, VAPIDSettingsKey, pn.keys)
			if err != nil {
				return nil, err
			}
		}
	}

	return pn, nil
}
