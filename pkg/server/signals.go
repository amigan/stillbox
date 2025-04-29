package server

import (
	"os"
	"os/signal"
	"syscall"

	"dynatron.me/x/stillbox/pkg/config"
	"github.com/rs/zerolog/log"
)

type hupper interface {
	HUP(*config.Config)
}

func (s *Server) huppers() []hupper {
	return []hupper{
		s.logger,
		s.auth,
		s.tgs,
		s.alerter,
		s.users,
		s.transcriber,
	}
}

func (s *Server) hupHandler() {
	s.hup = make(chan os.Signal, 1)
	go func() {
		for sig := range s.hup {
			log.Info().Msgf("received %s", sig)
			err := s.conf.ReadConfig()
			if err != nil {
				log.Error().Err(err).Msg("cannot read config")
				continue
			}

			hs := s.huppers()
			for _, h := range hs {
				h.HUP(&s.conf.Config)
			}
		}
	}()

	signal.Notify(s.hup, syscall.SIGHUP)
}
