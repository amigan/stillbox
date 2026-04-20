package notify

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"text/template"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/alerting/alert"
	"dynatron.me/x/stillbox/pkg/config"
)

// A Notiifer is the notification controller.
type Notifier interface {
	Send(ctx context.Context, alerts []alert.Alert) error
}

// A NotifierBackend actually dispatches the notiication to the recipients.
type NotifierBackend interface {
	// Dispatch sends the rendered alerts out to recipients.
	Dispatch(ctx context.Context, renderedAlerts *alert.RenderedAlertBatch) error
}

type notifier struct {
	backends []NotifierBackend
	subject  *template.Template
	alert    *template.Template

	baseURL *url.URL
}

func fmtTime(fmt string, t time.Time) string {
	return t.Format(fmt)
}

var alertFm = template.FuncMap{
	"fmtTime": fmtTime,
}

const (
	defaultBodyTemplStr = `{{ .TGName }}{{ if (and .Talkgroup .Talkgroup.AlphaTag) }} ({{ .Talkgroup.StringTag false -}}){{ end }} is active with a score of {{ f .Score.Score 4 }}! ({{ f .Score.RecentCount 0 }}/{{ .Score.Count }} recent calls)
{{- range .Context }}
{{ .Date | fmtTime "15:04:05" }} {{ .Transcript }}
{{- end }}`
	defaultSubjectTemplStr = `({{ .TGName }})`
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

func (n *notifier) makeAlertURL(al *alert.Alert) string {
	if al.URLTag == nil {
		return ""
	}

	return n.baseURL.JoinPath("alert", *al.URLTag).String()
}

type beRegFunc func(config.ConfigMap) (NotifierBackend, error)

var backendRegistry map[string]beRegFunc

func registerBackend(name string, fn beRegFunc) {
	if backendRegistry == nil {
		backendRegistry = make(map[string]beRegFunc)
	}

	backendRegistry[name] = fn
}

func (n *notifier) addBackend(be NotifierBackend) {
	n.backends = append(n.backends, be)
}

func (n *notifier) addService(cfg config.NotifyService) (err error) {
	befn, has := backendRegistry[cfg.Provider]
	if !has {
		return fmt.Errorf("unknown provider '%s'", cfg.Provider)
	}

	be, err := befn(cfg.Config)
	if err != nil {
		return err
	}

	n.addBackend(be)

	return nil
}

func (n *notifier) Send(ctx context.Context, alerts []alert.Alert) error {
	rab := &alert.RenderedAlertBatch{}
	rab.Alerts = make([]alert.RenderedAlert, 0, len(alerts))

	for i, al := range alerts {
		var subject, body bytes.Buffer
		err := n.subject.ExecuteTemplate(&subject, "subject", al)
		if err != nil {
			return err
		}

		err = n.alert.ExecuteTemplate(&body, "body", al)
		if err != nil {
			return err
		}

		rab.Alerts = append(rab.Alerts, alert.RenderedAlert{
			Alert:   &alerts[i],
			Subject: subject.String(),
			Body:    body.String(),
			URL:     n.makeAlertURL(&al),
		})

		if al.Score.Score > rab.TopScore {
			rab.TopScore = al.Score.Score
			rab.TopIdx = i
		}
	}

	for _, be := range n.backends {
		err := be.Dispatch(ctx, rab)
		if err != nil {
			return err
		}
	}

	return nil
}

func New(cfg config.Notify, baseURL *url.URL, pushSvc NotifierBackend) (*notifier, error) {
	n := &notifier{
		baseURL:  baseURL,
		backends: make([]NotifierBackend, 0, len(cfg.Backends)+1),
	}

	var err error
	switch cfg.SubjectTemplate {
	case nil:
		n.subject = defaultTemplate.Lookup("subject")
		if n.subject == nil {
			panic("subject template nil")
		}
	default:
		n.subject, err = template.New("subject").Funcs(common.FuncMap).Funcs(alertFm).Parse(*cfg.SubjectTemplate)
		if err != nil {
			return nil, err
		}
	}

	switch cfg.BodyTemplate {
	case nil:
		n.alert = defaultTemplate.Lookup("body")
		if n.alert == nil {
			panic("body template nil")
		}
	default:
		n.alert, err = template.New("body").Funcs(common.FuncMap).Funcs(alertFm).Parse(*cfg.BodyTemplate)
		if err != nil {
			return nil, err
		}
	}

	n.addBackend(pushSvc)

	for _, s := range cfg.Backends {
		err := n.addService(s)
		if err != nil {
			return nil, err
		}
	}

	return n, nil
}
