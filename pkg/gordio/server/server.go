package server

import (
	"log"
	"net/http"

	"dynatron.me/x/stillbox/pkg/gordio/config"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/jwtauth/v5"
)

type Server struct {
	conf *config.Config
	r    *chi.Mux
	jwt  *jwtauth.JWTAuth
}

func New(cfg *config.Config) (*Server, error) {
	r := chi.NewRouter()
	srv := &Server{
		conf: cfg,
		r:    r,
		jwt:  jwtauth.New("HS256", []byte(cfg.JWTSecret), nil),
	}
	_, tokenString, err := srv.jwt.Encode(map[string]interface{}{"user_id": 123})
	if err != nil {
		panic(err)
	}
	log.Printf("DEBUG token is %s", tokenString)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	srv.setupRoutes()

	return srv, nil
}

func (s *Server) Go() error {
	http.ListenAndServe(s.conf.Listen, s.r)
	return nil
}

