package server

import (
	"io"
	"io/fs"
	"os"
	"os/signal"
	"syscall"

	"dynatron.me/x/stillbox/pkg/gordio/config"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	LOGPERM fs.FileMode = 0600
)

type Logger struct {
	console io.Writer
	writers []io.Writer
	hup chan os.Signal
}

func NewLogger(cfg *config.Config) (*Logger, error) {
	l := &Logger{
		console: &zerolog.ConsoleWriter{Out: os.Stderr},
	}
	l.hup = make(chan os.Signal, 1)
	go func() {
		for sig := range l.hup {
			log.Logger = log.Output(l.console)
			log.Info().Msgf("received %s, closing and reopening logfiles", sig)
			l.Close()
			err := l.OpenLogs(cfg)
			if err != nil {
				log.Error().Err(err).Msg("error reopening logs")
				continue
			}

			log.Logger = log.Output(zerolog.MultiLevelWriter(l.writers...))
		}
	}()

	signal.Notify(l.hup, syscall.SIGHUP)

	err := l.OpenLogs(cfg)
	if err != nil {
		return nil, err
	}

	log.Logger = log.Output(zerolog.MultiLevelWriter(l.writers...))

	return l, nil
}

func (l *Logger) Close() {
	for _, lg := range l.writers {
		if _, isConsole := lg.(*zerolog.ConsoleWriter); isConsole {
			continue
		}

		if cl, isCloser := lg.(io.Closer); isCloser {
			err := cl.Close()
			if err != nil {
				log.Error().Err(err).Msg("closing writer")
			}
		}
	}

	l.writers = nil
}

func (l *Logger) OpenLogs(cfg *config.Config) error {
	l.writers = make([]io.Writer, 0, len(cfg.Log))
	for _, lc := range cfg.Log {
		level := zerolog.TraceLevel
		if lc.Level != nil {
			var err error

			level, err = zerolog.ParseLevel(*lc.Level)
			if err != nil {
				return err
			}
		}

		w := &zerolog.FilteredLevelWriter{
			Level: level,
		}

		switch lc.File {
		case nil:
			w.Writer = &zerolog.LevelWriterAdapter{Writer: l.console}
		default:
			f, err := os.OpenFile(*lc.File, os.O_WRONLY|os.O_CREATE, LOGPERM)
			if err != nil {
				return err
			}

			w.Writer = &zerolog.LevelWriterAdapter{
				Writer: f,
			}
		}

		l.writers = append(l.writers, w)
	}

	return nil
}
