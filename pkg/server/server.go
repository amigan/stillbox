package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"time"

	"dynatron.me/x/stillbox/pkg/alerting"
	"dynatron.me/x/stillbox/pkg/authn"
	"dynatron.me/x/stillbox/pkg/authz"
	"dynatron.me/x/stillbox/pkg/authz/policy"
	"dynatron.me/x/stillbox/pkg/calls/callstore"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/database/partman"
	"dynatron.me/x/stillbox/pkg/incidents/incstore"
	"dynatron.me/x/stillbox/pkg/metrics"
	"dynatron.me/x/stillbox/pkg/nexus"
	"dynatron.me/x/stillbox/pkg/notify"
	"dynatron.me/x/stillbox/pkg/rest"
	"dynatron.me/x/stillbox/pkg/services"
	"dynatron.me/x/stillbox/pkg/settings"
	"dynatron.me/x/stillbox/pkg/shares"
	"dynatron.me/x/stillbox/pkg/sinks"
	"dynatron.me/x/stillbox/pkg/sources"
	"dynatron.me/x/stillbox/pkg/stats"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
	"dynatron.me/x/stillbox/pkg/users"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
)

const shutdownTimeout = 5 * time.Second

type Server struct {
	auth        authn.Authn
	conf        *config.Configuration
	db          database.Store
	r           *chi.Mux
	sources     sources.Sources
	sinks       sinks.Sinks
	relayer     *sinks.RelayManager
	transcriber sinks.Transcriber
	nex         nexus.Nexus
	logger      *Logger
	alerter     alerting.Alerter
	notifier    notify.Notifier
	hup         chan os.Signal
	tgs         tgstore.Store
	rest        rest.APIRoot
	partman     partman.PartitionManager
	users       users.Store
	calls       callstore.Store
	incidents   incstore.Store
	share       shares.Service
	rbac        authz.RBAC
	stats       stats.Stats
	settings    settings.Store
	metrics     metrics.Metrics
	srvMetrics  srvMetrics
}

type srvMetrics struct {
	Requests      *prometheus.CounterVec `help:"Requests" labels:"code,method"`
	RequestMS     prometheus.Histogram   `help:"Request durations." buckets:"1,5,10,30,100,200,500"`
	IngestedCalls prometheus.Counter     `help:"Total ingested calls."`
}

func New(ctx context.Context, cfg *config.Configuration) (*Server, error) {
	logger, err := NewLogger(cfg.Log)
	if err != nil {
		return nil, err
	}

	met, err := metrics.NewMetrics(cfg.Metrics)
	if err != nil {
		return nil, err
	}

	db, err := database.NewClient(ctx, cfg.DB)
	if err != nil {
		return nil, err
	}

	r := chi.NewRouter()

	ust := users.NewStore(db)

	authenticator, err := authn.NewAuthn(cfg.Auth, met, ust)
	if err != nil {
		return nil, err
	}

	notifier, err := notify.New(cfg.Notify)
	if err != nil {
		return nil, err
	}

	tgCache := tgstore.NewCache(db, met)

	rbacSvc, err := authz.New(policy.Policy)
	if err != nil {
		return nil, err
	}

	callStore := callstore.NewStore(db)
	statsSvc := stats.NewStats(callStore, stats.DefaultExpiration)

	nex := nexus.New(tgCache, met)

	srv := &Server{
		auth:      authenticator,
		conf:      cfg,
		db:        db,
		r:         r,
		nex:       nex,
		logger:    logger,
		alerter:   alerting.New(cfg.Alerting, tgCache, alerting.WithNotifier(notifier)),
		notifier:  notifier,
		tgs:       tgCache,
		sinks:     sinks.NewSinkManager(),
		share:     shares.NewService(),
		users:     ust,
		metrics:   met,
		calls:     callStore,
		incidents: incstore.NewStore(),
		rbac:      rbacSvc,
		stats:     statsSvc,
		settings:  settings.New(settings.ConfigDefaults),
	}

	transcriber, err := sinks.NewTranscriber(srv.sinks, authenticator, srv.tgs, cfg.Transcription)
	if err != nil {
		return nil, err
	}

	api := rest.New(cfg.Server.BaseURL.URL(), nex, transcriber)
	srv.rest = api

	srv.metrics.Register("http", &srv.srvMetrics)

	if cfg.DB.Partition.Enabled {
		srv.partman, err = partman.New(db, cfg.DB.Partition)
		if err != nil {
			return nil, err
		}

		err = srv.partman.Check(ctx, time.Now())
		if err != nil {
			return nil, err
		}
	}

	srv.sinks.Register(sinks.NewDatabaseSink(db, tgCache), true)
	srv.sinks.Register(sinks.NewNexusSink(srv.nex), false)
	srv.sinks.Register(transcriber, false)

	if srv.alerter.Enabled() {
		srv.sinks.Register(srv.alerter, false)
	}

	srv.sources.Register("rdio-http", sources.NewRdioHTTP(authenticator, srv))

	relayer, err := sinks.NewRelayManager(srv.sinks, cfg.Relay)
	if err != nil {
		return nil, err
	}

	srv.relayer = relayer

	ctx = srv.fillCtx(ctx)

	r.Use(middleware.RequestID)

	if cfg.Server.UseXRealIP {
		r.Use(middleware.RealIP)
	}

	r.Use(srv.MetricsLogger())
	r.Use(ServerHeaderAdd)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   srv.conf.Server.CORS.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "Upgrade"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))

	err = srv.setupRoutes(ctx)
	if err != nil {
		return nil, err
	}

	if os.Getenv("STILLBOX_DUMP_ROUTES") == "true" {
		_ = chi.Walk(r, func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
			fmt.Printf("[%s]: '%s' has %d middlewares\n", method, route, len(middlewares))
			return nil
		})
	}

	return srv, nil
}

func (s *Server) fillCtx(ctx context.Context) context.Context {
	svc := services.New()
	ctx = services.CtxWith(ctx, svc)

	ctx = database.CtxWithDB(ctx, s.db)
	ctx = tgstore.CtxWithStore(ctx, s.tgs)
	ctx = users.CtxWithStore(ctx, s.users)
	ctx = callstore.CtxWithStore(ctx, s.calls)
	ctx = incstore.CtxWithStore(ctx, s.incidents)
	ctx = shares.CtxWithStore(ctx, s.share)
	ctx = authz.CtxWithRBAC(ctx, s.rbac)
	ctx = stats.CtxWithStats(ctx, s.stats)
	ctx = settings.CtxWithStore(ctx, s.settings)

	return ctx
}

func (s *Server) MetricsLogger() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			t1 := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			defer func() {
				if r := recover(); r != nil && r != http.ErrAbortHandler {
					log.Error().Interface("recover", r).Bytes("stack", debug.Stack()).Msg("incoming_request_panic")
					ww.WriteHeader(http.StatusInternalServerError)
				}
				status := ww.Status()
				dur := time.Since(t1)
				log.Info().Fields(map[string]any{
					"remote_addr": r.RemoteAddr,
					"path":        r.URL.Path,
					"proto":       r.Proto,
					"method":      r.Method,
					"user_agent":  r.UserAgent(),
					"status":      http.StatusText(status),
					"status_code": status,
					"bytes_in":    r.ContentLength,
					"bytes_out":   ww.BytesWritten(),
					"duration":    dur.String(),
					"reqID":       middleware.GetReqID(r.Context()),
				}).Msg("incoming_request")

				s.srvMetrics.Requests.WithLabelValues(strconv.Itoa(status), r.Method).Inc()

				// milliseconds
				s.srvMetrics.RequestMS.Observe(float64(dur.Microseconds()) / 1000)
			}()
			next.ServeHTTP(ww, r)
		}
		return http.HandlerFunc(fn)
	}
}

func (s *Server) Go(ctx context.Context, shutReq chan<- error) error {
	defer database.Close(s.db)

	s.hupHandler()

	ctx = s.fillCtx(ctx)

	httpSrv := &http.Server{
		Addr:    s.conf.Server.Listen,
		Handler: s.r,
	}
	var err error
	go func() {
		err = httpSrv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			shutReq <- err
		}
	}()

	go s.nex.Go(ctx)
	go s.alerter.Go(ctx)
	go s.share.Go(ctx)

	if pm := s.partman; pm != nil {
		go pm.Go(ctx)
	}

	<-ctx.Done()

	s.sinks.Shutdown()

	ctxShutdown, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := httpSrv.Shutdown(ctxShutdown); err != nil {
		log.Fatal().Err(err).Msg("shutdown failed")
	}
	if err == http.ErrServerClosed {
		err = nil
	}

	return err
}
