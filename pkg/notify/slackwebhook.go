package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"dynatron.me/x/stillbox/pkg/config"
	"github.com/go-viper/mapstructure/v2"
)

func init() {
	registerBackend("slackwebhook", NewSlackWebhookBackend)
}

type slackWebhookBackend struct {
	WebhookURL      string `mapstructure:"webhookURL"`
	Icon            string `mapstructure:"icon"`
	SubjectTemplate string `mapstructure:"subjectTemplate"`
	BodyTemplate    string `mapstructure:"bodyTemplate"`
	ConcatAlerts    bool   `mapstructure:"concatAlerts"`

	client *http.Client
}

func NewSlackWebhookBackend(cfg config.ConfigMap) (notifierBackend, error) {
	swc := &slackWebhookBackend{
		Icon:   "🚨",
		client: http.DefaultClient,
	}
	err := mapstructure.Decode(cfg, &swc)
	if err != nil {
		return nil, err
	}

	return swc, nil
}

func (be *slackWebhookBackend) Dispatch(ctx context.Context, ras *renderedAlertBatch) error {
	type Attachment struct {
		Title     string `json:"title"`
		Text      string `json:"text"`
		Fallback  string `json:"fallback"`
		Footer    string `json:"footer"`
		TitleLink string `json:"title_link"`
		Timestamp int64  `json:"ts"`
	}
	m := struct {
		Username    string       `json:"username"`
		Attachments []Attachment `json:"attachments"`
		IconEmoji   string       `json:"icon_emoji"`
	}{
		Username:  "Stillbox",
		IconEmoji: be.Icon,
	}

	now := time.Now().Unix()
	if be.ConcatAlerts {
		top := ras.top()
		if top == nil { // should not happen
			return errors.New("bad top index")
		}

		var body strings.Builder
		for i, ra := range ras.alerts {
			body.WriteString(ra.body)
			if len(ras.alerts) > 1 {
				if i != ras.topIdx {
					// only need URL if not in title
					body.WriteRune('\n')
					body.WriteString(ra.url)
				}
				if i < len(ras.alerts)-1 {
					body.WriteRune('\n')
					body.WriteRune('\n')
				}
			}
		}
		m.Attachments = []Attachment{
			{
				Title:     top.subject,
				Text:      body.String(),
				TitleLink: top.url,
				Timestamp: now,
			},
		}
	} else {
		m.Attachments = make([]Attachment, 0, len(ras.alerts))
		for _, ra := range ras.alerts {
			m.Attachments = append(m.Attachments, Attachment{
				Title:     ra.subject,
				Text:      ra.body,
				TitleLink: ra.url,
				Timestamp: now,
			})
		}
	}

	body := new(bytes.Buffer)
	err := json.NewEncoder(body).Encode(&m)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, be.WebhookURL, io.TeeReader(body, os.Stdout))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := be.client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("received webhook response status %d", resp.StatusCode)
	}

	return nil
}
