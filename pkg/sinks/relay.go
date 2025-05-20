package sinks

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"

	"dynatron.me/x/stillbox/internal/forms"
	"dynatron.me/x/stillbox/internal/version"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/config"
)

var (
	RelayUserAgent = version.HttpString("call-relay")
)

type RelayManager struct {
	xp     *http.Transport
	client *http.Client

	relays []*Relay
}

type Relay struct {
	config.Relay
	mgr  *RelayManager
	name string

	url *url.URL
}

func (r *Relay) Name() string {
	return r.name
}

func NewRelayManager(s Sinks, cfgs []config.Relay) (*RelayManager, error) {
	xp := http.DefaultTransport.(*http.Transport).Clone()
	xp.MaxIdleConnsPerHost = 10

	client := &http.Client{
		Transport: xp,
	}

	rm := &RelayManager{
		xp:     xp,
		client: client,
		relays: make([]*Relay, 0, len(cfgs)),
	}

	for i, cfg := range cfgs {
		rs, err := rm.newRelay(i, cfg)
		if err != nil {
			return nil, err
		}

		rm.relays = append(rm.relays, rs)

		s.Register(rs, cfg.Required)
	}

	return rm, nil
}

func (rs *RelayManager) newRelay(idx int, cfg config.Relay) (*Relay, error) {
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, err
	}

	if u.Path != "" && u.Path != "/" {
		return nil, fmt.Errorf("relay path in %s must be instance root", cfg.URL)
	}

	u = u.JoinPath("/api/call-upload")

	return &Relay{
		name:  fmt.Sprintf("relay%d:%s", idx, u.Host),
		Relay: cfg,
		url:   u,
		mgr:   rs,
	}, nil
}

func (s *Relay) Call(ctx context.Context, call *calls.Call) error {
	var buf bytes.Buffer
	body := multipart.NewWriter(&buf)

	err := forms.Marshal(call, body, forms.WithTag("relayOut"))
	if err != nil {
		return fmt.Errorf("relay form parse: %w", err)
	}

	err = body.WriteField("key", s.APIKey)
	if err != nil {
		return fmt.Errorf("relay set API key: %w", err)
	}

	body.Close()

	r, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url.String(), &buf)
	if err != nil {
		return fmt.Errorf("relay newrequest: %w", err)
	}

	r.Header.Set("Content-Type", body.FormDataContentType())
	r.Header.Set("User-Agent", RelayUserAgent)

	resp, err := s.mgr.client.Do(r)
	if err != nil {
		return fmt.Errorf("relay %s: %w", s.name, err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		et, _ := io.ReadAll(r.Body)
		return fmt.Errorf("relay %s: received HTTP %d (%s)", s.name, resp.StatusCode, string(et))
	}

	_, _ = io.Copy(io.Discard, resp.Body)

	return nil
}

func (s *Relay) SinkType() string {
	return "relay"
}
