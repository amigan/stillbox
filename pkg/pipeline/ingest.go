package pipeline

import (
	"context"
	"sync"
	"time"

	"dynatron.me/x/stillbox/pkg/alerting"
	"dynatron.me/x/stillbox/pkg/authn"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/calls/callstore"
	"dynatron.me/x/stillbox/pkg/calls/filter"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/metrics"
	"dynatron.me/x/stillbox/pkg/nexus"
	"dynatron.me/x/stillbox/pkg/pipeline/sinks"
	"dynatron.me/x/stillbox/pkg/pipeline/sources"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
)

type pipe struct {
	// protects ingest filters
	sync.Mutex

	sources sources.Sources
	sinks   sinks.Sinks
	relayer *sinks.RelayManager
	metrics pipeMetrics
	filters []ingestFilter
	tgstore tgstore.Store
}

type pipeMetrics struct {
	IngestedCallsCount prometheus.Counter `help:"Total ingested calls"`
	FilteredCallsCount prometheus.Counter `help:"Count of calls rejected by ingest filters"`
}

type Pipeline interface {
	HUPCtx(context.Context, *config.Config)
	PublicRoutes(chi.Router)
	Ingest(ctx context.Context, call *calls.Call) error
	Shutdown()
}

var _ Pipeline = (*pipe)(nil)

func New(
	ctx context.Context,
	authenticator authn.Authn,
	callStore callstore.Store,
	tgs tgstore.Store,
	met metrics.Metrics,
	alerter alerting.Alerter,
	nex nexus.Nexus,
	cfg *config.Configuration,
) (*pipe, error) {
	ctx = entities.CtxWithServiceSubject(ctx, "pipeline")

	sinkMgr := sinks.NewSinkManager()

	p := &pipe{
		sinks:   sinkMgr,
		tgstore: tgs,
	}

	p.sinks.Register(sinks.NewCallstoreSink(callStore, tgs), sinks.RequiredFlag)
	p.sinks.Register(sinks.NewNexusSink(nex))

	if alerter.Enabled() {
		p.sinks.Register(alerter)
	}

	p.sources.Register("rdio-http", sources.NewRdioHTTP(authenticator, p))

	relayer, err := sinks.NewRelayManager(p.sinks, met, cfg.Relay)
	if err != nil {
		return nil, err
	}

	p.filters, err = buildIngestFilters(ctx, cfg.Ingest)
	if err != nil {
		return nil, err
	}

	p.relayer = relayer

	met.Register("pipeline", &p.metrics)

	return p, nil
}

type ingestFilter struct {
	filter *filter.Filter
	dur    time.Duration
}

func buildIngestFilters(ctx context.Context, flt []config.IngestFilter) ([]ingestFilter, error) {
	if len(flt) < 1 {
		return nil, nil
	}

	flts := make([]ingestFilter, 0, len(flt))
	for _, f := range flt {
		filt, err := filter.FromMap(f.Match)
		if err != nil {
			return nil, err
		}

		err = filt.Recompile(ctx)
		if err != nil {
			return nil, err
		}

		flts = append(flts, ingestFilter{
			filter: filt,
			dur:    f.MinDuration,
		})
	}

	log.Debug().Int("count", len(flts)).Msg("compiled ingest filters")

	return flts, nil
}

func (p *pipe) testIngest(ctx context.Context, call *calls.Call) bool {
	p.Lock()
	defer p.Unlock()

	for _, f := range p.filters {
		if f.filter.Test(ctx, call) {
			return call.Duration.Duration() >= f.dur
		}
	}

	return true
}

func (p *pipe) Ingest(ctx context.Context, call *calls.Call) error {
	if !p.testIngest(ctx, call) {
		p.metrics.FilteredCallsCount.Add(1)
		return nil
	}

	ctx = context.WithoutCancel(ctx)
	err := p.sinks.EmitCall(ctx, call)
	if err != nil {
		return err
	}

	p.metrics.IngestedCallsCount.Inc()

	return nil
}

func (p *pipe) HUPCtx(ctx context.Context, cfg *config.Config) {
	p.Lock()
	defer p.Unlock()
	ctx = entities.CtxWithServiceSubject(ctx, "pipeline")

	p.relayer.HUP(cfg)

	flt, err := buildIngestFilters(ctx, cfg.Ingest)
	if err != nil {
		log.Error().Err(err).Msg("build ingest filters failed, keeping old set")
	} else {
		log.Info().Msg("rebuilt ingest filters")
		p.filters = flt
	}
}

func (p *pipe) Shutdown() {
	p.sinks.Shutdown()
}

func (p *pipe) PublicRoutes(r chi.Router) {
	p.sources.PublicRoutes(r)
}
