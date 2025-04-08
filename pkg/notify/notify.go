package notify

import (
	"bytes"
	"context"
	"fmt"
	stdhttp "net/http"
	"text/template"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/alerting/alert"
	"dynatron.me/x/stillbox/pkg/config"

	"github.com/go-viper/mapstructure/v2"
	"github.com/nikoksr/notify"
	"github.com/nikoksr/notify/service/http"
)

type Notifier interface {
	Send(ctx context.Context, alerts []alert.Alert) error
}

type backend struct {
	*notify.Notify
	subject *template.Template
	body    *template.Template
}

type notifier struct {
	backends []backend
}

func highest(a []alert.Alert) string {
	if len(a) < 1 {
		return "none"
	}

	top := a[0]
	for _, a := range a {
		if a.Score.Score > top.Score.Score {
			top = a
		}
	}

	return top.TGName
}

func fmtTime(fmt string, t time.Time) string {
	return t.Format(fmt)
}

var alertFm = template.FuncMap{
	"highest": highest,
	"fmtTime": fmtTime,
}

const (
	defaultBodyTemplStr = `{{ range . -}}
{{ .TGName }}{{ if (and .Talkgroup .Talkgroup.AlphaTag) }} ({{ .Talkgroup.StringTag false -}}){{ end }} is active with a score of {{ f .Score.Score 4 }}! ({{ f .Score.RecentCount 0 }}/{{ .Score.Count }} recent calls)
{{- range .Context }}
{{ .Date | fmtTime "03:04:05" }} {{ .Transcript }}
{{- end -}}

{{ end -}}`
	defaultSubjectTemplStr = `Stillbox Alert ({{ highest . }})`
)

var (
	defaultTemplate *template.Template
)

func init() {
	defaultTemplate = template.New("notification")
	defaultTemplate.Funcs(common.FuncMap).Funcs(alertFm)
	_, err := defaultTemplate.New("body").Parse(defaultBodyTemplStr)
	if err != nil {
		panic(err)
	}

	_, err = defaultTemplate.New("subject").Parse(defaultSubjectTemplStr)
	if err != nil {
		panic(err)
	}
}

// Send renders and sends the Alerts.
func (b *backend) Send(ctx context.Context, alerts []alert.Alert) (err error) {
	var subject, body bytes.Buffer
	err = b.subject.ExecuteTemplate(&subject, "subject", alerts)
	if err != nil {
		return err
	}

	err = b.body.ExecuteTemplate(&body, "body", alerts)
	if err != nil {
		return err
	}

	err = b.Notify.Send(ctx, subject.String(), body.String())
	if err != nil {
		return err
	}

	return nil
}

func buildSlackWebhookPayload(cfg *slackWebhookConfig) func(string, string) any {
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
	WebhookURL      string `mapstructure:"webhookURL"`
	Icon            string `mapstructure:"icon"`
	MessageURL      string `mapstructure:"messageURL"`
	SubjectTemplate string `mapstructure:"subjectTemplate"`
	BodyTemplate    string `mapstructure:"bodyTemplate"`
}

func (n *notifier) addService(cfg config.NotifyService) (err error) {
	be := backend{}

	switch cfg.SubjectTemplate {
	case nil:
		be.subject = defaultTemplate.Lookup("subject")
		if be.subject == nil {
			panic("subject template nil")
		}
	default:
		be.subject, err = template.New("subject").Funcs(common.FuncMap).Funcs(alertFm).Parse(*cfg.SubjectTemplate)
		if err != nil {
			return err
		}
	}

	switch cfg.BodyTemplate {
	case nil:
		be.body = defaultTemplate.Lookup("body")
		if be.body == nil {
			panic("body template nil")
		}
	default:
		be.body, err = template.New("body").Funcs(common.FuncMap).Funcs(alertFm).Parse(*cfg.BodyTemplate)
		if err != nil {
			return err
		}
	}

	be.Notify = notify.New()

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
			BuildPayload: buildSlackWebhookPayload(swc),
		})
		be.UseServices(hs)
	default:
		return fmt.Errorf("unknown provider '%s'", cfg.Provider)
	}

	n.backends = append(n.backends, be)

	return nil
}

func (n *notifier) Send(ctx context.Context, alerts []alert.Alert) error {
	for _, be := range n.backends {
		err := be.Send(ctx, alerts)
		if err != nil {
			return err
		}
	}

	return nil
}

func New(cfg config.Notify) (*notifier, error) {
	n := new(notifier)

	for _, s := range cfg {
		err := n.addService(s)
		if err != nil {
			return nil, err
		}
	}

	return n, nil
}
