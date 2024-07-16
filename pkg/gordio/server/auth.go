package server

import (
	"context"
	"errors"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"time"

	"dynatron.me/x/stillbox/pkg/gordio/database"

	"github.com/go-chi/jwtauth/v5"
	"github.com/rs/zerolog/log"
)

type claims map[string]interface{}

func (s *Server) Authenticated(r *http.Request) (claims, bool) {
	// TODO: check IP against ACL, or conf.Public, and against map of routes
	tok, cl, err := jwtauth.FromContext(r.Context())
	return cl, err != nil && tok != nil
}

var (
	ErrLoginFailed = errors.New("Login failed")
)

func (s *Server) Login(ctx context.Context, username, password string) (token string, err error) {
	q := database.New(database.FromCtx(ctx))
	users, err := q.GetUsers(ctx)
	if err != nil {
		log.Error().Err(err).Msg("getUsers failed")
		return "", ErrLoginFailed
	}

	var found *database.User

	for _, u := range users {
		if u.Username == username {
			found = &u
		}
	}

	if found == nil {
		_ = bcrypt.CompareHashAndPassword([]byte("lol@timing"), []byte(password))
		return "", ErrLoginFailed
	} else {
		err = bcrypt.CompareHashAndPassword([]byte(found.Password), []byte(password))
		if err != nil {
			return "", ErrLoginFailed
		}
	}

	return s.NewToken(found.ID), nil
}

func (s *Server) NewToken(uid int32) string {
	claims := claims{
		"user_id": uid,
	}
	jwtauth.SetExpiryIn(claims, time.Hour*24*30) // one month
	_, tokenString, err := s.jwt.Encode(claims)
	if err != nil {
		panic(err)
	}
	return tokenString
}
