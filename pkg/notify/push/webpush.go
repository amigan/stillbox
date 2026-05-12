package push

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/alerting/alert"
	"dynatron.me/x/stillbox/pkg/database"
	"github.com/SherClockHolmes/webpush-go"
	"github.com/hashicorp/go-multierror"
)

type webpushSender struct {
	pn                *pushNotifier
	badgeURL, iconURL string
}

type NotificationAction struct {
	Action string `json:"action,omitzero"`
	Title  string `json:"title,omitzero"`
}

type Notification struct {
	Actions            []NotificationAction `json:"actions,omitempty"`
	Badge              *string              `json:"badge,omitempty"`
	Body               *string              `json:"body,omitempty"`
	Data               any                  `json:"data,omitempty"`
	Direction          *string              `json:"dir,omitempty"`
	Icon               *string              `json:"icon,omitempty"`
	Image              *string              `json:"image,omitempty"`
	Language           *string              `json:"lang,omitempty"`
	Navigate           *string              `json:"navigate,omitempty"`
	Renotify           *bool                `json:"renotify,omitempty"`
	RequireInteraction *bool                `json:"requireInteraction,omitempty"`
	Silent             *bool                `json:"silent,omitempty"`
	Tag                *string              `json:"tag,omitempty"`
	Timestamp          *jsontypes.Time      `json:"timestamp,omitempty"`
	Title              string               `json:"title"` // required
	Vibrate            []int                `json:"vibrate,omitempty"`
}

func ReadWebPushSubscription(r io.Reader) (*WebPushSubscription, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	sub := new(WebPushSubscription)
	err = json.Unmarshal(raw, sub)
	if err != nil {
		return nil, err
	}

	sub.raw = raw

	return sub, nil
}

type WebNotification struct {
	Notification Notification `json:"notification"`
}

const (
	ClientStillbox = "stillbox"
)

func (wps *webpushSender) ralToNotification(a *alert.RenderedAlert, client *string) WebNotification {
	var rend notificationRenderer
	if client != nil {
		switch *client {
		case ClientStillbox:
			fallthrough
		default:
			rend = &stillboxNotificationRenderer{}
		}
	} else {
		rend = &stillboxNotificationRenderer{}
	}

	return rend.Render(wps, a)
}

type notificationRenderer interface {
	Render(*webpushSender, *alert.RenderedAlert) WebNotification
}

type stillboxNotificationRenderer struct{}

func (*stillboxNotificationRenderer) Render(wps *webpushSender, a *alert.RenderedAlert) WebNotification {
	return WebNotification{
		Notification: Notification{
			Title:     a.Subject,
			Body:      &a.Body,
			Timestamp: (*jsontypes.Time)(&a.Timestamp),
			Navigate:  &a.URL,
			Badge:     &wps.badgeURL,
			Icon:      &wps.iconURL,
			Data: map[string]any{
				"onActionClick": map[string]any{
					"default": map[string]any{
						"operation": "focusLastFocusedOrOpen",
						"url":       a.URL,
					},
				},
			},
		},
	}
}

func (vk *vapidKeys) generate() (err error) {
	vk.Private, vk.Public, err = webpush.GenerateVAPIDKeys()
	return err
}

type WebPushSubscription struct {
	webpush.Subscription
	Expiration *time.Time `json:"expirationTime,omitempty"`
	Client     *string    `json:"client,omitempty"`

	raw json.RawMessage `json:"-"`
}

func (wps *webpushSender) Send(ctx context.Context, subs []database.GetWebPushSubscriptionsSubscribedRow, al *alert.RenderedAlert) error {
	var me error

	for _, sub := range subs {
		rendAlert, err := json.Marshal(wps.ralToNotification(al, sub.Client))
		if err != nil {
			me = multierror.Append(me, err)
			continue
		}

		var wpSub webpush.Subscription
		err = json.Unmarshal(sub.Subscription, &wpSub)
		if err != nil {
			me = multierror.Append(me, err)
			continue
		}

		resp, err := webpush.SendNotificationWithContext(ctx, rendAlert, &wpSub, &webpush.Options{
			Subscriber:      wps.pn.baseURL.String(), // this is poorly named
			VAPIDPublicKey:  wps.pn.keys.Public,
			VAPIDPrivateKey: wps.pn.keys.Private,
			TTL:             120,
		})
		if err != nil {
			me = multierror.Append(me, err)
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			err := wps.pn.db.DeletePushSubscriptionByID(ctx, sub.ID)
			if err != nil {
				me = multierror.Append(me, err)
			}
			continue
		}
	}

	return nil
}

func newWebpushSender(pn *pushNotifier) *webpushSender {
	return &webpushSender{
		pn:       pn,
		badgeURL: pn.baseURL.JoinPath("icons", "icon-96x96.png").String(),
		iconURL:  pn.baseURL.JoinPath("icons", "icon-128x128.png").String(),
	}
}
