package webpush

import (
	"context"
	"encoding/json"

	"dynatron.me/x/stillbox/pkg/alerting/alert"
	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/hashicorp/go-multierror"
)

type Sender interface {
	Send(ctx context.Context, subs [][]byte, al *alert.RenderedAlert) error
}

type webpushSender struct {
	pn *pushNotifier
}

func (wps *webpushSender) Send(ctx context.Context, subs [][]byte, al *alert.RenderedAlert) error {
	var me error
	rendAlert, err := json.Marshal(al)
	if err != nil {
		return err
	}

	for _, subRaw := range subs {
		var sub webpush.Subscription
		err := json.Unmarshal(subRaw, &sub)
		if err != nil {
			me = multierror.Append(me, err)
			continue
		}

		resp, err := webpush.SendNotificationWithContext(ctx, rendAlert, &sub, &webpush.Options{
			Subscriber:      wps.pn.subject, // this is poorly named
			VAPIDPublicKey:  wps.pn.keys.Public,
			VAPIDPrivateKey: wps.pn.keys.Private,
			TTL:             120,
		})
		if err != nil {
			me = multierror.Append(me, err)
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			err := wps.pn.DeleteSubscription(ctx, subRaw)
			if err != nil {
				me = multierror.Append(me, err)
			}
			continue
		}
	}

	return nil
}

func newWebpushSender(pn *pushNotifier) *webpushSender {
	return &webpushSender{pn}
}
