package server

import (
	"context"
	"net/http"
	"os"
	"time"

	"dynatron.me/x/stillbox/pkg/alerting"
	"dynatron.me/x/stillbox/pkg/auth"
	"dynatron.me/x/stillbox/pkg/calls/callstore"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/database/partman"
	"dynatron.me/x/stillbox/pkg/incidents/incstore"
	"dynatron.me/x/stillbox/pkg/nexus"
	"dynatron.me/x/stillbox/pkg/notify"
	"dynatron.me/x/stillbox/pkg/rbac"
	"dynatron.me/x/stillbox/pkg/rest"
	"dynatron.me/x/stillbox/pkg/shares"
	"dynatron.me/x/stillbox/pkg/sinks"
	"dynatron.me/x/stillbox/pkg/sources"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
	"dynatron.me/x/stillbox/pkg/users"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/rs/zerolog/log"
)

const shutdownTimeout = 5 * time.Second

type Server struct {
	auth      *auth.Auth
	conf      *config.Configuration
	db        database.Store
	r         *chi.Mux
	sources   sources.Sources
	sinks     sinks.Sinks
	relayer   *sinks.RelayManager
	nex       *nexus.Nexus
	logger    *Logger
	alerter   alerting.Alerter
	notifier  notify.Notifier
	hup       chan os.Signal
	tgs       tgstore.Store
	rest      rest.APIRoot
	partman   partman.PartitionManager
	users     users.Store
	calls     callstore.Store
	incidents incstore.Store
	share     shares.Service
	rbac      rbac.RBAC
}

func New(ctx context.Context, cfg *config.Configuration) (*Server, error) {
	logger, err := NewLogger(cfg.Log)
	if err != nil {
		return nil, err
	}

	db, err := database.NewClient(ctx, cfg.DB)
	if err != nil {
		return nil, err
	}

	r := chi.NewRouter()

	ust := users.NewStore(db)

	authenticator := auth.NewAuthenticator(cfg.Auth, ust)

	notifier, err := notify.New(cfg.Notify)
	if err != nil {
		return nil, err
	}

	tgCache := tgstore.NewCache(db)
	api := rest.New(cfg.BaseURL.URL())

	rbacSvc, err := rbac.New()
	if err != nil {
		return nil, err
	}

	srv := &Server{
		auth:      authenticator,
		conf:      cfg,
		db:        db,
		r:         r,
		nex:       nexus.New(),
		logger:    logger,
		alerter:   alerting.New(cfg.Alerting, tgCache, alerting.WithNotifier(notifier)),
		notifier:  notifier,
		tgs:       tgCache,
		sinks:     sinks.NewSinkManager(),
		rest:      api,
		share:     shares.NewService(),
		users:     ust,
		calls:     callstore.NewStore(db),
		incidents: incstore.NewStore(),
		rbac:      rbacSvc,
	}

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

	srv.sinks.Register("database", sinks.NewDatabaseSink(db, tgCache), true)
	srv.sinks.Register("nexus", sinks.NewNexusSink(srv.nex), false)

	if srv.alerter.Enabled() {
		srv.sinks.Register("alerting", srv.alerter, false)
	}

	srv.sources.Register("rdio-http", sources.NewRdioHTTP(authenticator, srv))

	relayer, err := sinks.NewRelayManager(srv.sinks, cfg.Relay)
	if err != nil {
		return nil, err
	}

	srv.relayer = relayer

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(RequestLogger())
	r.Use(ServerHeaderAdd)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   srv.conf.CORS.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "Upgrade"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))
	srv.setupRoutes()

	return srv, nil
}

func (s *Server) fillCtx(ctx context.Context) context.Context {
	ctx = database.CtxWithDB(ctx, s.db)
	ctx = tgstore.CtxWithStore(ctx, s.tgs)
	ctx = users.CtxWithStore(ctx, s.users)
	ctx = callstore.CtxWithStore(ctx, s.calls)
	ctx = incstore.CtxWithStore(ctx, s.incidents)
	ctx = shares.CtxWithStore(ctx, s.share)
	ctx = rbac.CtxWithRBAC(ctx, s.rbac)

	return ctx
}

func (s *Server) Go(ctx context.Context) error {
	defer database.Close(s.db)

	s.installHupHandler()

	ctx = s.fillCtx(ctx)

	httpSrv := &http.Server{
		Addr:    s.conf.Listen,
		Handler: s.r,
	}

	go s.nex.Go(ctx)
	go s.alerter.Go(ctx)
	go s.share.Go(ctx)

	if pm := s.partman; pm != nil {
		go pm.Go(ctx)
	}

	var err error
	go func() {
		err = httpSrv.ListenAndServe()
	}()
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
