package sinks

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"dynatron.me/x/stillbox/internal/version"
	"dynatron.me/x/stillbox/pkg/auth"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/pb"
	"dynatron.me/x/stillbox/pkg/talkgroups/filter"
	"github.com/go-viper/mapstructure/v2"
	"google.golang.org/protobuf/proto"
)

var (
	ErrInvalidTranscriberTransport = errors.New("invalid transcriber transport type")

	TranscribeUserAgent = version.HttpString("call-transcribe")
)

type TranscriptionManager struct {
	xp     *http.Transport
	client *http.Client
	auth   *auth.Auth

	transcribers []*Transcriber
}

type Transcriber struct {
	Filter  *filter.TalkgroupFilter
	AtLeast time.Duration

	transport transcribeTransport

	mgr  *TranscriptionManager
	Name string
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
		rs, err := tm.newTranscriber(i, cfg)
		if err != nil {
			return nil, err
		}

		tm.transcribers = append(tm.transcribers, rs)

		s.Register(rs.Name, rs, false)
	}

	return tm, nil
}

type transcribeTransport interface {
	fmt.Stringer
	Dispatch(ctx context.Context, call *calls.Call) error
}

func (rs *TranscriptionManager) newTranscriber(idx int, cfg config.Transcription) (*Transcriber, error) {
	t := &Transcriber{
		AtLeast: time.Second * time.Duration(cfg.AtLeastSeconds),
		mgr:     rs,
	}
	var err error

	switch cfg.Type {
	case "http":
		t.transport, err = newHttpTranscriber(cfg.Config, rs)
		if err != nil {
			return nil, err
		}
	default:
		return nil, ErrInvalidTranscriberTransport
	}

	t.Name = fmt.Sprintf("transcriber%d:%s:%s", idx, cfg.Type, t.transport.String())

	if cfg.Filter != nil {
		filt, err := filter.FromMap(cfg.Filter)
		if err != nil {
			return nil, err
		}

		t.Filter = filt
	}

	return t, nil
}

type httpTranscriberTransport struct {
	URL          string `yaml:"url"`
	CallbackBase string `yaml:"callbackBase"`

	url          *url.URL
	callbackBase *url.URL
	mgr          *TranscriptionManager
}

func newHttpTranscriber(cfg config.ConfigMap, mgr *TranscriptionManager) (*httpTranscriberTransport, error) {
	htt := &httpTranscriberTransport{
		mgr: mgr,
	}
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Metadata:         nil,
		Result:           htt,
		TagName:          "yaml",
		WeaklyTypedInput: true,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.TextUnmarshallerHookFunc(),
		),
	})
	if err != nil {
		return nil, err
	}
	err = dec.Decode(cfg)
	if err != nil {
		return nil, err
	}

	u, err := url.Parse(htt.URL)
	if err != nil {
		return nil, err
	}

	cbBase, err := url.Parse(cfg.CallbackBase)
	if err != nil {
		return nil, err
	}

	cbBase, err := url.Parse(htt.CallbackBase)
	if err != nil {
		return nil, err
	}
	var err error

	switch cfg.Type {
	case "http":
		t.transport, err = newHttpTranscriber(cfg.Config, rs)
		if err != nil {
			return nil, err
		}
	default:
		return nil, ErrInvalidTranscriberTransport
	}

	t.Name = fmt.Sprintf("transcriber%d:%s:%s", idx, cfg.Type, t.transport.String())

	htt.url = u
	htt.callbackBase = cbBase

	return htt, nil
}

func (h *httpTranscriberTransport) String() string {
	return fmt.Sprintf("http:%s", h.url.Hostname())
}

func (h *httpTranscriberTransport) Dispatch(ctx context.Context, call *calls.Call) error {
	token := h.mgr.auth.NewCallToken(call.ID.String())

	callbackURL := *h.callbackBase

	u, err := url.Parse(htt.URL)
	if err != nil {
		return nil, err
	}

	u.Path = "/call"

	cbBase, err := url.Parse(htt.CallbackBase)
	if err != nil {
		return nil, err
	}

	htt.url = u
	htt.callbackBase = cbBase

	return htt, nil
}

func (h *httpTranscriberTransport) String() string {
	return fmt.Sprintf("http:%s", h.url.Hostname())
}

func (h *httpTranscriberTransport) Dispatch(ctx context.Context, call *calls.Call) error {
	token := h.mgr.auth.NewCallToken(call.ID.String())

	callbackURL := *h.callbackBase

	callbackURL.Path = "/api/call/" + call.ID.String() + "/transcript"

	cRq := &pb.CallTranscribeRequest{
		Call:     call.ToPB(),
		Callback: callbackURL.String(),
		Token:    token,
	}

	msg, err := proto.Marshal(cRq)
	if err != nil {
		return err
	}

	rdr := bytes.NewBuffer(msg)

	r, err := http.NewRequestWithContext(ctx, http.MethodPost, h.url.String(), rdr)
	if err != nil {
		return fmt.Errorf("transcribe newrequest: %w", err)
	}

	r.Header.Set("Content-Type", "application/x-protobuf")
	r.Header.Set("User-Agent", TranscribeUserAgent)

	resp, err := h.mgr.client.Do(r)
	if err != nil {
		return err
	}

	defer r.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received HTTP %d", resp.StatusCode)
	}

	return nil
}

func (s *Transcriber) Call(ctx context.Context, call *calls.Call) error {
	if call.Duration < calls.CallDuration(s.AtLeast) || !s.Filter.Test(ctx, call) {
		return nil
	}

	err := s.transport.Dispatch(ctx, call)
	if err != nil {
		return fmt.Errorf("transcribe:%s: %w", s.Name, err)
	}

	return nil
}

func (s *Transcriber) SinkType() string {
	return "transcribe"
}
