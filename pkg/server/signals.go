package server

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"dynatron.me/x/stillbox/pkg/config"
	"github.com/rs/zerolog/log"
)

type hupper interface {
	HUP(*config.Config)
}

type hupSvc interface {
	HUPCtx(context.Context, *config.Config)
}

func (s *Server) huppers() []hupper {
	return []hupper{
		s.logger,
		s.auth,
		s.tgs,
		s.alerter,
		s.users,
	}
}

func (s *Server) hupSvcs() []hupSvc {
	return []hupSvc{
		s.pipeline,
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

	ctx := s.fillCtx(context.Background())

	for _, h := range s.hupSvcs() {
		h.HUPCtx(ctx, &s.conf.Config)
	}
}

// installSignalHandler is for non-terminating signals. Terminating signals are in internal/cmd/serve.
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
