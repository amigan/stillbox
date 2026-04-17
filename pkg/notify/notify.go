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

type Notifier interface {
	Send(ctx context.Context, alerts []alert.Alert) error
}

type notifierBackend interface {
	Dispatch(ctx context.Context, renderedAlerts *renderedAlertBatch) error
}

type notifier struct {
	backends []notifierBackend
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
	defaultSubjectTemplStr = `Stillbox Alert ({{ .TGName }})`
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

type renderedAlertBatch struct {
	alerts   []renderedAlert
	topIdx   int
	topScore float64
}

func (r *renderedAlertBatch) top() *renderedAlert {
	if r.topIdx > len(r.alerts)-1 {
		return nil
	}

	return &r.alerts[r.topIdx]
}

type renderedAlert struct {
	subject string
	body    string
	url     string
}

func (n *notifier) makeAlertURL(al *alert.Alert) string {
	if al.URLTag == nil {
		return ""
	}

	return n.baseURL.JoinPath("alert", *al.URLTag).String()
}

type beRegFunc func(config.ConfigMap) (notifierBackend, error)

var backendRegistry map[string]beRegFunc

func registerBackend(name string, fn beRegFunc) {
	if backendRegistry == nil {
		backendRegistry = make(map[string]beRegFunc)
	}

	backendRegistry[name] = fn
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

	n.backends = append(n.backends, be)

	return nil
}

func (n *notifier) Send(ctx context.Context, alerts []alert.Alert) error {
	rab := &renderedAlertBatch{}
	rab.alerts = make([]renderedAlert, 0, len(alerts))

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

		rab.alerts = append(rab.alerts, renderedAlert{
			subject: subject.String(),
			body:    body.String(),
			url:     n.makeAlertURL(&al),
		})

		if al.Score.Score > rab.topScore {
			rab.topScore = al.Score.Score
			rab.topIdx = i
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

func New(cfg config.Notify, baseURL *url.URL) (*notifier, error) {
	n := &notifier{
		baseURL: baseURL,
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

	for _, s := range cfg.Backends {
		err := n.addService(s)
		if err != nil {
			return nil, err
		}
	}

	return n, nil
}
