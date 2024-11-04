package notify

import (
	"fmt"
	stdhttp "net/http"
	"time"

	"dynatron.me/x/stillbox/pkg/config"

	"github.com/go-viper/mapstructure/v2"
	"github.com/nikoksr/notify"
	"github.com/nikoksr/notify/service/http"
)

type Notifier interface {
	notify.Notifier
}

type notifier struct {
	*notify.Notify
}

func (n *notifier) buildSlackWebhookPayload(cfg *slackWebhookConfig) func(string, string) any {

	type Attachment struct {
		Title     string `json:"title"`
		Text      string `json:"text"`
		Fallback  string `json:"fallback"`
		Footer    string `json:"footer"`
		TitleLink string `json:"title_link"`
		Timestamp int64  `json:"ts"`
	}
	return func(subject, message string) any {
		m := struct {
			Username    string       `json:"username"`
			Attachments []Attachment `json:"attachments"`
			IconEmoji   string       `json:"icon_emoji"`
		}{
			Username: "Stillbox",
			Attachments: []Attachment{
				{
					Title:     subject,
					Text:      message,
					TitleLink: cfg.MessageURL,
					Timestamp: time.Now().Unix(),
				},
			},
			IconEmoji: cfg.Icon,
		}

		return m
	}
}

type slackWebhookConfig struct {
	WebhookURL string `mapstructure:"webhookURL"`
	Icon       string `mapstructure:"icon"`
	MessageURL string `mapstructure:"messageURL"`
}

func (n *notifier) addService(cfg config.NotifyService) error {
	switch cfg.Provider {
	case "slackwebhook":
		swc := &slackWebhookConfig{
			Icon: "🚨",
		}
		err := mapstructure.Decode(cfg.Config, &swc)
		if err != nil {
			return err
		}
		hs := http.New()
		hs.AddReceivers(&http.Webhook{
			ContentType:  "application/json",
			Header:       make(stdhttp.Header),
			Method:       stdhttp.MethodPost,
			URL:          swc.WebhookURL,
			BuildPayload: n.buildSlackWebhookPayload(swc),
		})
		n.UseServices(hs)
	default:
		return fmt.Errorf("unknown provider '%s'", cfg.Provider)
	}

	return nil
}

func New(cfg config.Notify) (Notifier, error) {
	n := &notifier{
		Notify: notify.NewWithServices(),
	}

	for _, s := range cfg {
		err := n.addService(s)
		if err != nil {
			return nil, err
		}
	}

	return n, nil
}
