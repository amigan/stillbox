package server

import (
	"context"
	"net/http"

	"dynatron.me/x/stillbox/pkg/gordio/auth"
	"dynatron.me/x/stillbox/pkg/gordio/config"
	"dynatron.me/x/stillbox/pkg/gordio/database"
	"dynatron.me/x/stillbox/pkg/gordio/nexus"
	"dynatron.me/x/stillbox/pkg/gordio/sinks"
	"dynatron.me/x/stillbox/pkg/gordio/sources"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

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
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
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

func (s *Server) Go() error {
	defer s.db.Close()

	ctx, cancel := context.WithCancel(database.CtxWithDB(context.Background(), s.db))
	defer cancel()

	go s.nex.Go(ctx)

	return http.ListenAndServe(s.conf.Listen, s.r)
}
