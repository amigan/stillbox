package push

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"

	"dynatron.me/x/stillbox/pkg/alerting/alert"
	"dynatron.me/x/stillbox/pkg/authz"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/notify"
	"dynatron.me/x/stillbox/pkg/settings"
	"dynatron.me/x/stillbox/pkg/talkgroups"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
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
	WebPushSubscribe(ctx context.Context, sub *WebPushSubscription) error

	// Subscribe stores a new subscription set for the user.
	Subscribe(ctx context.Context, sub *SubscriptionSet) error

	// Unsubscribe unsubscribes a subscription set for the user.
	Unsubscribe(ctx context.Context, sub *SubscriptionSet) error

	// Subscriptions lists all subscriptions for the user.
	Subscriptions(ctx context.Context) (*SubscriptionSet, error)
}

type SubscriptionSet struct {
	Talkgroups talkgroups.IDs `json:"talkgroups,omitempty"`
	Systems    []int32        `json:"systems,omitempty"`

	// UnsubscribeAll is only valid for unsubscribing. It is a no-op otherwise.
	UnsubscribeAll *bool `json:"unsubscribeAll,omitempty"`
}

func (pn *pushNotifier) Unsubscribe(ctx context.Context, sub *SubscriptionSet) error {
	u, err := users.UserCheck(ctx, authz.UseResource(entities.ResourcePushSub), "delete")
	if err != nil {
		return err
	}

	err = pn.db.InTx(ctx, func(s database.Store) error {
		if sub.UnsubscribeAll != nil && *sub.UnsubscribeAll {
			_, err := s.UnsubscribeAllSystems(ctx, u.ID.Int())
			if err != nil {
				return err
			}

			_, err = s.UnsubscribeAllTalkgroups(ctx, u.ID.Int())
			return err
		}
		tgs := tgstore.TGsToDBTGs(sub.Talkgroups)
		if sub.Talkgroups != nil {
			_, err := s.UnsubscribeTalkgroups(ctx, u.ID.Int(), tgs)
			if err != nil {
				return err
			}
		}

		if sub.Systems != nil {
			_, err := s.UnsubscribeSystems(ctx, u.ID.Int(), sub.Systems)
			if err != nil {
				return err
			}
		}

		return nil
	})

	return err

}

func (pn *pushNotifier) Subscribe(ctx context.Context, sub *SubscriptionSet) error {
	u, err := users.UserCheck(ctx, authz.UseResource(entities.ResourcePushSub), "create")
	if err != nil {
		return err
	}

	err = pn.db.InTx(ctx, func(s database.Store) error {
		tgs := sub.Talkgroups.Tuples()
		if sub.Talkgroups != nil {
			err := s.SubscribeTalkgroups(ctx, u.ID.Int(), tgs[0], tgs[1])
			if err != nil {
				return err
			}
		}

		if sub.Systems != nil {
			err := s.SubscribeSystems(ctx, u.ID.Int(), sub.Systems)
			if err != nil {
				return err
			}
		}

		return nil
	})

	return err
}

func (pn *pushNotifier) Subscriptions(ctx context.Context) (*SubscriptionSet, error) {
	u, err := users.UserCheck(ctx, authz.UseResource(entities.ResourcePushSub), "read")
	if err != nil {
		return nil, err
	}

	subSet := &SubscriptionSet{}

	return subSet, pn.db.InTx(ctx, func(db database.Store) error {
		tgSubs, err := db.GetTalkgroupSubscriptions(ctx, u.ID.Int())
		if err != nil {
			return err
		}

		if tgSubs != nil { // preserve nil
			subSet.Talkgroups = make(talkgroups.IDs, 0, len(tgSubs))
			for _, tg := range tgSubs {
				subSet.Talkgroups = append(subSet.Talkgroups, talkgroups.ID{
					System:    uint32(tg.SystemID),
					Talkgroup: uint32(tg.TGID),
				})
			}
		}

		subSet.Systems, err = db.GetSystemSubscriptions(ctx, u.ID.Int())

		return err
	})
}

type pushNotifier struct {
	settings settings.Store
	db       database.Store
	keys     vapidKeys
	webPush  Sender
	baseURL  *url.URL
}

type Sender interface {
	Send(ctx context.Context, subs []database.GetWebPushSubscriptionsSubscribedRow, al *alert.RenderedAlert) error
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
		notifySubs, err := pn.db.GetWebPushSubscriptionsSubscribed(ctx, int32(al.TGID.System), int32(al.TGID.Talkgroup))
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

func (pn *pushNotifier) WebPushSubscribe(ctx context.Context, sub *WebPushSubscription) error {
	user, err := users.UserCheck(ctx, authz.UseResource(entities.ResourceWebPushSub), "create")
	if err != nil {
		return err
	}

	return pn.db.CreateWebPushSubscription(ctx, user.ID.Int(), sub.Expiration, sub.raw)
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
