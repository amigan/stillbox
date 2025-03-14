package sinks

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"

	"dynatron.me/x/stillbox/internal/version"
	"dynatron.me/x/stillbox/pkg/auth"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/pb"
	"google.golang.org/protobuf/proto"
)

type TranscriptionManager struct {
	xp     *http.Transport
	client *http.Client
	auth   *auth.Auth

	transcribers []*Transcriber
}

type Transcriber struct {
	config.Transcription
	mgr  *TranscriptionManager
	Name string
	auth *auth.Auth

	url *url.URL
}

func NewTranscriptionManager(s Sinks, a *auth.Auth, cfgs []config.Transcription) (*TranscriptionManager, error) {
	xp := http.DefaultTransport.(*http.Transport).Clone()
	xp.MaxIdleConnsPerHost = 10

	client := &http.Client{
		Transport: xp,
	}

	tm := &TranscriptionManager{
		auth:         a,
		xp:           xp,
		client:       client,
		transcribers: make([]*Transcriber, 0, len(cfgs)),
	}

	for i, cfg := range cfgs {
		rs, err := tm.newTranscriber(i, a, cfg)
		if err != nil {
			return nil, err
		}

		tm.transcribers = append(tm.transcribers, rs)

		s.Register(rs.Name, rs, false)
	}

	return tm, nil
}

func (rs *TranscriptionManager) newTranscriber(idx int, a *auth.Auth, cfg config.Transcription) (*Transcriber, error) {
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, err
	}

	u.Path = "/call"

	return &Transcriber{
		Name:          fmt.Sprintf("transcriber%d:%s", idx, u.Host),
		Transcription: cfg,
		url:           u,
		mgr:           rs,
		auth:          a,
	}, nil
}

func (s *Transcriber) Call(ctx context.Context, call *calls.Call) error {
	token := s.auth.NewCallToken(call.ID.String())
	callbackURL := ""

	cRq := &pb.CallTranscribeRequest{
		Call:     call.ToPB(),
		Callback: callbackURL,
		Token:    token,
	}

	cm, err := proto.Marshal(cRq)
	if err != nil {
		return err
	}

	rdr := bytes.NewBuffer(cm)

	r, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url.String(), rdr)
	if err != nil {
		return fmt.Errorf("transcribe newrequest: %w", err)
	}

	r.Header.Set("Content-Type", "application/x-protobuf")
	r.Header.Set("User-Agent", version.HttpString("call-transcribe"))

	resp, err := s.mgr.client.Do(r)
	if err != nil {
		return fmt.Errorf("transcribe %s: %w", s.Name, err)
	}

	defer r.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("transcribe %s: received HTTP %d", s.Name, resp.StatusCode)
	}

	return nil
}

func (s *Transcriber) SinkType() string {
	return "transcribe"
}
