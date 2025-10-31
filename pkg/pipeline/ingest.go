package pipeline

import (
	"context"

	"dynatron.me/x/stillbox/pkg/alerting"
	"dynatron.me/x/stillbox/pkg/authn"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/calls/callstore"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/metrics"
	"dynatron.me/x/stillbox/pkg/nexus"
	"dynatron.me/x/stillbox/pkg/pipeline/sinks"
	"dynatron.me/x/stillbox/pkg/pipeline/sources"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
)

type pipe struct {
	sources     sources.Sources
	sinks       sinks.Sinks
	transcriber sinks.Transcriber
	relayer     *sinks.RelayManager
	metrics     pipeMetrics
	filters     []filter
}

type pipeMetrics struct {
	IngestedCallsCount prometheus.Counter `help:"Total ingested calls"`
}

type Pipeline interface {
	Transcriber() sinks.Transcriber
	HUP(*config.Config)
	PublicRoutes(chi.Router)
	Shutdown()
}

var _ Pipeline = (*pipe)(nil)

func New(
	authenticator authn.Authn,
	callStore callstore.Store,
	tgs tgstore.Store,
	met metrics.Metrics,
	alerter alerting.Alerter,
	nex nexus.Nexus,
	cfg *config.Configuration,
) (*pipe, error) {
	sinkMgr := sinks.NewSinkManager()
	transcriber, err := sinks.NewTranscriber(sinkMgr, authenticator, tgs, met, cfg.Transcription)
	if err != nil {
		return nil, err
	}
	p := &pipe{
		sinks:       sinkMgr,
		transcriber: transcriber,
	}

	p.sinks.Register(sinks.NewCallstoreSink(callStore, tgs), sinks.RequiredFlag)
	p.sinks.Register(sinks.NewNexusSink(nex))
	p.sinks.Register(p.transcriber)

	if alerter.Enabled() {
		p.sinks.Register(alerter)
	}

	p.sources.Register("rdio-http", sources.NewRdioHTTP(authenticator, p))

	relayer, err := sinks.NewRelayManager(p.sinks, met, cfg.Relay)
	if err != nil {
		return nil, err
	}

	err = p.buildIngestFilters(cfg.Ingest)
	if err != nil {
		return nil, err
	}

	p.relayer = relayer

	met.Register("pipeline", &p.metrics)

	return p, nil
}

type filter struct {
}

func (p *pipe) buildIngestFilters(_ []config.IngestFilter) error {
	return nil
}

func (p *pipe) Ingest(ctx context.Context, call *calls.Call) error {
	ctx = context.WithoutCancel(ctx)
	err := p.sinks.EmitCall(ctx, call)
	if err != nil {
		return err
	}

	p.metrics.IngestedCallsCount.Inc()

	return nil
}

func (p *pipe) Transcriber() sinks.Transcriber {
	return p.transcriber
}

func (p *pipe) HUP(cfg *config.Config) {
	p.transcriber.HUP(cfg)
	p.relayer.HUP(cfg)
}

func (p *pipe) Shutdown() {
	p.sinks.Shutdown()
}

func (p *pipe) PublicRoutes(r chi.Router) {
	p.sources.PublicRoutes(r)
}
