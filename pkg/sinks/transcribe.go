package sinks

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"dynatron.me/x/stillbox/internal/version"
	"dynatron.me/x/stillbox/pkg/authn"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/pb"
	"dynatron.me/x/stillbox/pkg/talkgroups/filter"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
	"github.com/go-viper/mapstructure/v2"
	"github.com/gohugoio/hashstructure"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"
)

var (
	ErrInvalidTranscriberTransport = errors.New("invalid transcriber transport type")

	TranscribeUserAgent = version.HttpString("call-transcribe")
)

// TranscriptionManager is the Sink that dispatches transcription jobs to Transcribers.
type TranscriptionManager struct {
	sync.RWMutex
	xp     *http.Transport
	client *http.Client
	auth   authn.Authn
	tgst   tgstore.Store
	sinks  Sinks

	cfgHash uint64

	transcribers []*Transcriber
}

// Transcriber is a collection of transports that operate on calls matching their Filter.
type Transcriber struct {
	Filter  *filter.TalkgroupFilter
	AtLeast time.Duration

	transport transcribeTransport

	mgr  *TranscriptionManager
	name string
}

func (txs *Transcriber) Name() string {
	return txs.name
}

func (s *Transcriber) Call(ctx context.Context, call *calls.Call) error {
	if call.Duration < calls.CallDuration(s.AtLeast) || !s.Filter.Test(ctx, call) || !call.ShouldStore() {
		return nil
	}

	err := s.transport.Dispatch(ctx, call)
	if err != nil {
		return fmt.Errorf("transcribe:%s: %w", s.name, err)
	}

	return nil
}

func (s *Transcriber) SinkType() string {
	return "transcribe"
}

func NewTranscriptionManager(s Sinks, a authn.Authn, tgst tgstore.Store, cfgs []config.Transcription) (*TranscriptionManager, error) {
	xp := http.DefaultTransport.(*http.Transport).Clone()
	xp.MaxIdleConnsPerHost = 10

	client := &http.Client{
		Transport: xp,
	}

	cfgHash, err := hashstructure.Hash(cfgs, nil)
	if err != nil {
		return nil, err
	}

	tm := &TranscriptionManager{
		auth:    a,
		xp:      xp,
		tgst:    tgst,
		client:  client,
		sinks:   s,
		cfgHash: cfgHash,
	}
	tm.Lock()
	defer tm.Unlock()

	err = tm.fillTxmsFromCfgs(cfgs)
	if err != nil {
		return nil, err
	}

	return tm, nil
}

func (tm *TranscriptionManager) fillTxmsFromCfgs(cfgs []config.Transcription) error {
	tm.transcribers = make([]*Transcriber, 0, len(cfgs))

	for i, cfg := range cfgs {
		rs, err := tm.newTranscriber(i, cfg)
		if err != nil {
			return err
		}

		tm.transcribers = append(tm.transcribers, rs)

		tm.sinks.Register(rs, false)
	}

	return nil
}

type transcribeTransport interface {
	fmt.Stringer
	Dispatch(ctx context.Context, call *calls.Call) error
}

// newTranscriber requires the caller take a lock.
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

	t.name = fmt.Sprintf("transcriber%d:%s:%s", idx, cfg.Type, t.transport.String())

	if cfg.Filter != nil {
		filt, err := filter.FromMap(cfg.Filter)
		if err != nil {
			return nil, err
		}

		t.Filter = filt
		t.mgr.tgst.RegisterFilter(t.Filter)
	}

	return t, nil
}

func (tm *TranscriptionManager) HUP(cfg *config.Config) {
	tm.Lock()
	defer tm.Unlock()

	hash, err := hashstructure.Hash(cfg.Transcription, nil)
	if err != nil {
		log.Error().Err(err).Msg("hup transcription config hash")
		return
	}

	if hash == tm.cfgHash {
		return
	}

	tm.cfgHash = 0 // set to invalid state in case we fail

	log.Info().Msg("reloading transcription config")

	for _, txr := range tm.transcribers {
		tm.tgst.UnregisterFilter(txr.Filter)
		tm.sinks.Unregister(txr)
	}

	err = tm.fillTxmsFromCfgs(cfg.Transcription)
	if err != nil {
		log.Error().Err(err).Msg("transcription reload failed")
		return
	}

	tm.cfgHash = hash
}

// httpTranscriberTransport is a transcription worker accepting jobs via HTTP.
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
	return h.url.Hostname()
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
		et, _ := io.ReadAll(r.Body)
		return fmt.Errorf("received HTTP %d body %s", resp.StatusCode, string(et))
	}

	io.Copy(io.Discard, resp.Body)

	return nil
}
