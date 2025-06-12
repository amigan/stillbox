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

	"dynatron.me/x/stillbox/internal/robin"
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
	ErrNoCallAudio                 = errors.New("no call audio")

	TranscribeUserAgent = version.HttpString("call-transcribe")
)

type workers robin.Robin[transcribeTransport]

// Transcriber is the Sink that dispatches transcription jobs to Transcribers.
type Transcriber interface {
	Sink
	UnfilteredCall(ctx context.Context, call *calls.Call) error
	HUP(*config.Config)
}

func (*transcriber) SinkType() string {
	return "transcriber"
}

func (*transcriber) Name() string {
	return "transcriber"
}

type transcriber struct {
	Filter  *filter.TalkgroupFilter
	AtLeast time.Duration

	sync.RWMutex
	xp     *http.Transport
	client *http.Client
	auth   authn.Authn
	tgst   tgstore.Store
	sinks  Sinks

	cfgHash uint64

	workers workers
}

func (s *transcriber) Call(ctx context.Context, call *calls.Call) error {
	if s.workers == nil || call.Duration < calls.CallDuration(s.AtLeast) || !s.Filter.Test(ctx, call) || !call.ShouldStore() {
		return nil
	}

	s.RLock()
	defer s.RUnlock()

	return s.workers.Next().Dispatch(ctx, call)
}

func (s *transcriber) UnfilteredCall(ctx context.Context, call *calls.Call) error {
	s.RLock()
	defer s.RUnlock()

	return s.workers.Next().Dispatch(ctx, call)
}

func NewTranscriber(s Sinks, a authn.Authn, tgst tgstore.Store, cfg config.Transcription) (*transcriber, error) {
	xp := http.DefaultTransport.(*http.Transport).Clone()
	xp.MaxIdleConnsPerHost = 10

	client := &http.Client{
		Transport: xp,
	}

	cfgHash, err := hashstructure.Hash(cfg, nil)
	if err != nil {
		return nil, err
	}

	t := &transcriber{
		AtLeast: time.Second * time.Duration(cfg.AtLeastSeconds),
		auth:    a,
		xp:      xp,
		tgst:    tgst,
		client:  client,
		sinks:   s,
		cfgHash: cfgHash,
	}

	err = t.initFilter(cfg)
	if err != nil {
		return nil, err
	}

	err = t.makeWorkers(cfg)
	if err != nil {
		return nil, err
	}

	return t, nil
}

func (t *transcriber) initFilter(cfg config.Transcription) error {
	if cfg.Filter == nil {
		return nil
	}

	filt, err := filter.FromMap(cfg.Filter)
	if err != nil {
		return err
	}

	t.Filter = filt
	t.tgst.RegisterFilter(t.Filter)

	return nil
}

type transcribeTransport interface {
	fmt.Stringer
	Dispatch(ctx context.Context, call *calls.Call) error
}

type transportConstructor func(*transcriber, config.ConfigMap) (transcribeTransport, error)

// newTranscriber requires the caller take a lock.
func (t *transcriber) makeWorkers(cfg config.Transcription) error {
	if len(cfg.Workers) < 1 {
		t.workers = nil
		return nil
	}
	wk := map[string]transportConstructor{
		"http": newHttpTranscriber,
	}
	w := make([]transcribeTransport, 0, len(cfg.Workers))
	for _, wc := range cfg.Workers {
		txp, has := wk[wc.Type]
		if !has {
			return fmt.Errorf("%w: %s", ErrInvalidTranscriberTransport, wc.Type)
		}

		txport, err := txp(t, wc.Config)
		if err != nil {
			return fmt.Errorf("transcribe %s: %w", wc.Type, err)
		}

		w = append(w, txport)
	}

	t.workers = robin.New(w)

	return nil
}

func (tm *transcriber) HUP(cfg *config.Config) {
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

	tm.tgst.UnregisterFilter(tm.Filter)

	err = tm.makeWorkers(cfg.Transcription)
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
	mgr          *transcriber
}

func (h *httpTranscriberTransport) String() string {
	return h.url.String()
}

func newHttpTranscriber(mgr *transcriber, cfg config.ConfigMap) (transcribeTransport, error) {
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

func (h *httpTranscriberTransport) Dispatch(ctx context.Context, call *calls.Call) error {
	if call == nil || len(call.Audio) == 0 {
		return ErrNoCallAudio
	}

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

	_, _ = io.Copy(io.Discard, resp.Body)

	return nil
}
