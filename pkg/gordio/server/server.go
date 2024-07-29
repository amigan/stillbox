package server

import (
	"net/http"

	"dynatron.me/x/stillbox/pkg/gordio/config"
	"dynatron.me/x/stillbox/pkg/gordio/database"
	"dynatron.me/x/stillbox/pkg/gordio/ingestors"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
)

type Server struct {
	conf *config.Config
	db   *database.DB
	r    *chi.Mux
	jwt  *jwtauth.JWTAuth
	hi   *ingestors.HTTPIngestor
}

func New(cfg *config.Config) (*Server, error) {
	db, err := database.NewClient(cfg.DB)
	if err != nil {
		return nil, err
	}

	r := chi.NewRouter()
	srv := &Server{
		conf: cfg,
		db:   db,
		r:    r,
		jwt:  jwtauth.New("HS256", []byte(cfg.JWTSecret), nil),
		hi:   ingestors.NewHTTPIngestor(),
	}
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
