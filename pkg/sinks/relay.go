package sinks

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"

	"dynatron.me/x/stillbox/internal/forms"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/config"
)

type RelaySink struct {
	config.Relay

	url *url.URL
}

func MakeRelaySinks(s *Sinks, cfgs []config.Relay) error {
	for i, cfg := range cfgs {
		rs, err := NewRelaySink(cfg)
		if err != nil {
			return err
		}

		sinkName := fmt.Sprintf("relay%d:%s", i, rs.url.Host)
		s.Register(sinkName, rs, cfg.Required)
	}

	return nil
}

func NewRelaySink(cfg config.Relay) (*RelaySink, error) {
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, err
	}

	if u.Path != "" && u.Path != "/" {
		return nil, fmt.Errorf("relay path in %s must be instance root", cfg.URL)
	}

	u = u.JoinPath("/api/call-upload")

	return &RelaySink{
		Relay: cfg,
		url:   u,
	}, nil
}

func (s *RelaySink) Call(ctx context.Context, call *calls.Call) error {
	var buf bytes.Buffer
	body := multipart.NewWriter(&buf)

	err := forms.Marshal(call, body)
	if err != nil {
		return fmt.Errorf("relay form parse: %w", err)
	}
	body.Close()

	r, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url.String(), &buf)
	if err != nil {
		return fmt.Errorf("relay newrequest: %w", err)
	}

	r.Header.Set("Content-Type", body.FormDataContentType())

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return fmt.Errorf("relay: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("relay: received HTTP %d", resp.StatusCode)
	}

	return nil
}

func (s *RelaySink) SinkType() string {
	return "relay"
}
