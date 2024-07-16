package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"dynatron.me/x/stillbox/pkg/gordio/database"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"github.com/rs/zerolog/log"
)

func (s *Server) dbTx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, err := s.db.Begin(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Error().Err(err).Msg("tx open failed")
		}

		r = r.WithContext(database.CtxWithTx(r.Context(), tx))

		defer func(ctx context.Context) {
			if rec := recover(); rec != nil {
				var err error
				switch r := rec.(type) {
				case error:
					err = r
				default:
					err = fmt.Errorf("%v", r)
				}
				w.WriteHeader(http.StatusInternalServerError)
				log.Error().Err(err).Msg("tx rollback due to panic")
				tx.Rollback(ctx)
			}
		}(r.Context())

		err = next.ServeHTTP(w, r)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Error().Err(err).Msg("tx rollback due to error")
			tx.Rollback(r.Context())
			return
		}

		err = tx.Commit(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Error().Err(err).Msg("tx commit failed")
		}
	})
}

func (s *Server) setupRoutes() {
	r := s.r
	r.Use(s.dbTx)

	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(s.jwt))
		r.Use(jwtauth.Authenticator(s.jwt))

	})

	r.Group(func(r chi.Router) {
		r.Use(rateLimiter())
		r.Use(render.SetContentType(render.ContentTypeJSON))
		// public routes
		r.Post("/auth", s.routeAuth)
	})

	r.Group(func(r chi.Router) {
		r.Use(rateLimiter())
		r.Use(jwtauth.Verifier(s.jwt))

		// optional auth routes

		r.Get("/", s.routeIndex)
	})
}

func rateLimiter() func(http.Handler) http.Handler {
	return httprate.LimitByRealIP(100, 1*time.Minute)
}

func (s *Server) routeIndex(w http.ResponseWriter, r *http.Request) {
	if cl, authenticated := s.Authenticated(r); authenticated {
		w.Write([]byte("Hello " + cl["user"].(string) + "\n"))
	}
	w.Write([]byte("Welcome to gordio\n"))
}

func (s *Server) routeAuth(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	username, password := r.PostFormValue("username"), r.PostFormValue("password")
	if username == "" || password == "" {
		http.Error(w, "blank credentials", http.StatusBadRequest)
		return
	}

	tok, err := s.Login(r.Context(), username, password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: "jwt",
		Value: tok,
		HttpOnly: true,
		Secure: true,
		Domain: s.conf.Domain,
	})

	jr := struct {
		JWT string `json:"jwt"`
	}{
		JWT: tok,
	}
	render.JSON(w, r, &jr)
}
