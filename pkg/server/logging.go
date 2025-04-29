package server

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"runtime/debug"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/config"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	LOGPERM fs.FileMode = 0600
)

type Logger struct {
	console io.Writer
	writers []io.Writer
	files   []*os.File
	cfg     []config.Logger

	lastFieldName string
	noColor       bool
}

func (l *Logger) consoleWriter() *zerolog.ConsoleWriter {
	return &zerolog.ConsoleWriter{
		Out:              os.Stderr,
		TimeFormat:       common.TimeFormat,
		FormatFieldName:  l.fieldNameFormat,
		FormatFieldValue: l.fieldValueFormat,
	}
}

func NewLogger(cfg []config.Logger) (*Logger, error) {
	l := &Logger{
		cfg: cfg,
	}

	err := l.OpenLogs(cfg)
	if err != nil {
		return nil, err
	}

	l.Install()

	return l, nil
}

func (l *Logger) HUP(cfg *config.Config) {
	l.cfg = cfg.Log

	log.Logger = log.Output(l.console)
	log.Info().Msg("closing and reopening logfiles")
	l.Close()
	err := l.OpenLogs(l.cfg)
	if err != nil {
		log.Error().Err(err).Msg("error reopening logs")
		return
	}

	l.Install()
}

func (l *Logger) Install() {
	log.Logger = log.Output(zerolog.MultiLevelWriter(l.writers...))
}

func (l *Logger) Close() {
	for _, lg := range l.files {
		lg.Close()
	}

	l.writers = nil
	l.files = nil
}

func (l *Logger) OpenLogs(cfg []config.Logger) error {
	l.writers = make([]io.Writer, 0, len(cfg))
	for _, lc := range cfg {
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
			w.Writer = &zerolog.LevelWriterAdapter{Writer: l.consoleWriter()}
		default:
			f, err := os.OpenFile(*lc.File, os.O_APPEND|os.O_WRONLY|os.O_CREATE, LOGPERM)
			if err != nil {
				return err
			}

			l.files = append(l.files, f)

			w.Writer = &zerolog.LevelWriterAdapter{
				Writer: f,
			}
		}

		l.writers = append(l.writers, w)
	}

	return nil
}

func RequestLogger() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			t1 := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			defer func() {
				if r := recover(); r != nil && r != http.ErrAbortHandler {
					log.Error().Interface("recover", r).Bytes("stack", debug.Stack()).Msg("incoming_request_panic")
					ww.WriteHeader(http.StatusInternalServerError)
				}
				log.Info().Fields(map[string]any{
					"remote_addr": r.RemoteAddr,
					"path":        r.URL.Path,
					"proto":       r.Proto,
					"method":      r.Method,
					"user_agent":  r.UserAgent(),
					"status":      http.StatusText(ww.Status()),
					"status_code": ww.Status(),
					"bytes_in":    r.ContentLength,
					"bytes_out":   ww.BytesWritten(),
					"duration":    time.Since(t1).String(),
					"reqID":       middleware.GetReqID(r.Context()),
				}).Msg("incoming_request")
			}()
			next.ServeHTTP(ww, r)
		}
		return http.HandlerFunc(fn)
	}
}

//nolint:unused
const (
	colorBlack = iota + 30
	colorRed
	colorGreen
	colorYellow
	colorBlue
	colorMagenta
	colorCyan
	colorWhite
	colorNone

	colorBold     = 1
	colorDarkGray = 90
)

func (l *Logger) fieldNameFormat(i any) string {
	l.lastFieldName = fmt.Sprint(i)
	return l.colorize(l.lastFieldName+"=", colorCyan)
}

func (l *Logger) fieldValueFormat(i any) string {
	color := colorNone
	switch l.lastFieldName {
	case "method":
		color = colorMagenta
	case "reqID":
		color = colorYellow
	case "duration":
		color = colorBlue
	}

	l.lastFieldName = ""

	if color == colorNone {
		return fmt.Sprint(i)
	}

	return l.colorize(i, color)
}

// colorize returns the string s wrapped in ANSI code c, unless disabled is true or c is 0.
func (l *Logger) colorize(s any, c int) string {
	if l.noColor {
		return fmt.Sprintf("%v", s)
	}
	return fmt.Sprintf("\x1b[%dm%v\x1b[0m", c, s)
}
