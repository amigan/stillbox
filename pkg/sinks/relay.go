package sinks

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"sync"

	"dynatron.me/x/stillbox/internal/forms"
	"dynatron.me/x/stillbox/internal/version"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/metrics"
	"github.com/gohugoio/hashstructure"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
)

var (
	RelayUserAgent = version.HttpString("call-relay")
)

type RelayManager struct {
	sync.RWMutex
	cfgHash uint64
	xp      *http.Transport
	client  *http.Client

	sinks Sinks

	relays []*Relay

	metrics relayManagerMetrics
}

type relayManagerMetrics struct {
	RelayedSuccess *prometheus.CounterVec `help:"Calls successfully relayed." labels:"url"`
	RelayedFailed  *prometheus.CounterVec `help:"Failed relay attempts." labels:"url"`
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

func NewRelayManager(s Sinks, met metrics.Metrics, cfgs []config.Relay) (*RelayManager, error) {
	xp := http.DefaultTransport.(*http.Transport).Clone()
	xp.MaxIdleConnsPerHost = 10

	client := &http.Client{
		Transport: xp,
	}

	rm := &RelayManager{
		xp:     xp,
		client: client,
		sinks:  s,
	}

	relays, err := rm.build(cfgs)
	if err != nil {
		return nil, err
	}

	rm.cfgHash, err = hashstructure.Hash(cfgs, nil)
	if err != nil {
		return nil, err
	}

	met.Register("relay", &rm.metrics)

	rm.Lock() // since we are about to register, rm may be subject to concurent access
	defer rm.Unlock()

	rm.register(relays)

	return rm, nil
}

func (rm *RelayManager) build(cfgs []config.Relay) ([]*Relay, error) {
	r := make([]*Relay, 0, len(cfgs))
	for i, cfg := range cfgs {
		rs, err := rm.newRelay(i, cfg)
		if err != nil {
			return nil, err
		}

		r = append(r, rs)
	}

	return r, nil
}

func (rm *RelayManager) register(relays []*Relay) {
	rm.relays = relays
	for _, r := range rm.relays {
		var flags []Flags
		if r.Required {
			flags = []Flags{RequiredFlag}
		}

		rm.sinks.Register(r, flags...)
	}
}

func (rm *RelayManager) unregister() {
	for _, r := range rm.relays {
		rm.sinks.Unregister(r)
	}

	rm.relays = nil
}

func (rs *RelayManager) HUP(cfg *config.Config) {
	rs.Lock()
	defer rs.Unlock()

	newHash, err := hashstructure.Hash(cfg.Relay, nil)
	if err != nil {
		log.Error().Err(err).Msg("relaymanager HUP config hash")
		return
	}

	if newHash == rs.cfgHash {
		return
	}

	log.Info().Msg("relay config changed, reloading")
	relays, err := rs.build(cfg.Relay)
	if err != nil {
		log.Error().Err(err).Msg("relay config fail")
		return
	}

	rs.unregister()
	rs.register(relays)
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

func (s *Relay) Call(ctx context.Context, call *calls.Call) (err error) {
	defer func() {
		if err != nil {
			s.mgr.metrics.RelayedFailed.WithLabelValues(s.URL).Inc()
		}
	}()

	var buf bytes.Buffer
	body := multipart.NewWriter(&buf)

	err = forms.Marshal(call, body, forms.WithTag("relayOut"))
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

	s.mgr.metrics.RelayedSuccess.WithLabelValues(s.URL).Inc()

	return nil
}

func (s *Relay) SinkType() string {
	return "relay"
}
