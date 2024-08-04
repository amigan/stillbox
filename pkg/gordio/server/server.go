package server

import (
	"net/http"

	"dynatron.me/x/stillbox/pkg/gordio/auth"
	"dynatron.me/x/stillbox/pkg/gordio/config"
	"dynatron.me/x/stillbox/pkg/gordio/database"
	"dynatron.me/x/stillbox/pkg/gordio/nexus"
	"dynatron.me/x/stillbox/pkg/gordio/sinks"
	"dynatron.me/x/stillbox/pkg/gordio/sources"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
)

type Server struct {
	auth    auth.Authenticator
	conf    *config.Config
	db      *database.DB
	r       *chi.Mux
	sources sources.Sources
	sinks   sinks.Sinks
	nex     *nexus.Nexus
}

func New(cfg *config.Config) (*Server, error) {
	db, err := database.NewClient(cfg.DB)
	if err != nil {
		return nil, err
	}

	r := chi.NewRouter()
	authenticator := auth.NewAuthenticator(cfg.JWTSecret, cfg.Domain)
	srv := &Server{
		auth: authenticator,
		conf: cfg,
		db:   db,
		r:    r,
		nex:  nexus.New(),
	}

	srv.sinks.Register("database", sinks.NewDatabaseSink(db))
	srv.sources.Register("rdio-http", sources.NewRdioHTTP(authenticator, srv))

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	srv.setupRoutes()

	return srv, nil
}

func (s *Server) Go() error {
	defer s.db.Close()

	http.ListenAndServe(s.conf.Listen, s.r)
	return nil
}
