package server

import (
	"context"
	"net/http"
	"time"

	"dynatron.me/x/stillbox/pkg/gordio/auth"
	"dynatron.me/x/stillbox/pkg/gordio/config"
	"dynatron.me/x/stillbox/pkg/gordio/database"
	"dynatron.me/x/stillbox/pkg/gordio/nexus"
	"dynatron.me/x/stillbox/pkg/gordio/sinks"
	"dynatron.me/x/stillbox/pkg/gordio/sources"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/rs/zerolog/log"
)

const shutdownTimeout = 5 * time.Second

type Server struct {
	auth    auth.Authenticator
	conf    *config.Config
	db      *database.DB
	r       *chi.Mux
	sources sources.Sources
	sinks   sinks.Sinks
	nex     *nexus.Nexus
	logger  *Logger
}

func New(cfg *config.Config) (*Server, error) {
	logger, err := NewLogger(cfg)
	if err != nil {
		return nil, err
	}

	db, err := database.NewClient(cfg.DB)
	if err != nil {
		return nil, err
	}

	r := chi.NewRouter()
	authenticator := auth.NewAuthenticator(cfg.Auth)
	srv := &Server{
		auth:   authenticator,
		conf:   cfg,
		db:     db,
		r:      r,
		nex:    nexus.New(),
		logger: logger,
	}

	srv.sinks.Register("database", sinks.NewDatabaseSink(srv.db), true)
	srv.sinks.Register("nexus", sinks.NewNexusSink(srv.nex), false)
	srv.sources.Register("rdio-http", sources.NewRdioHTTP(authenticator, srv))

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(RequestLogger())
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

func (s *Server) Go(ctx context.Context) error {
	defer s.db.Close()

	ctx = database.CtxWithDB(ctx, s.db)

	httpSrv := &http.Server{
		Addr:    s.conf.Listen,
		Handler: s.r,
	}

	go s.nex.Go(ctx)

	var err error
	go func() {
		err = httpSrv.ListenAndServe()
	}()
	<-ctx.Done()

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
