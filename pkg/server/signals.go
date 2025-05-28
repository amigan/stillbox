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
		s.relayer,
	}
}

func (s *Server) sighup() {
	err := s.conf.ReadConfig()
	if err != nil {
		log.Error().Err(err).Msg("cannot read config")
		return
	}

	hs := s.huppers()
	for _, h := range hs {
		h.HUP(&s.conf.Config)
	}
}

// installSignalHandler is for non-terminating signals. Terminating signals are in pkg/cmd/serve.
func (s *Server) installSignalHandlers() {
	s.signals = make(chan os.Signal, 1)
	go func() {
		for sig := range s.signals {
			log.Info().Msgf("received %s", sig)
			switch sig {
			case syscall.SIGHUP:
				s.sighup()
			}
		}
	}()

	signal.Notify(s.signals, syscall.SIGHUP)
}
