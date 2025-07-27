package serve

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/server"

	"github.com/urfave/cli/v3"
)

type ServeOptions struct {
	cfg *config.Configuration
}

func Command(cfg *config.Configuration) *cli.Command {
	opts := makeOptions(cfg)
	serveCmd := &cli.Command{
		Name:   "serve",
		Usage:  "starts the " + common.AppName + " server",
		Action: common.Action(opts),
	}

	return serveCmd
}

func makeOptions(cfg *config.Configuration) *ServeOptions {
	return &ServeOptions{
		cfg: cfg,
	}
}

func (o *ServeOptions) Options(_ context.Context, _ *cli.Command) error {
	return nil
}

func (o *ServeOptions) Execute() error {
	ctx, cancel := context.WithCancelCause(context.Background())

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer func() {
		signal.Stop(sig)
		cancel(nil)
	}()

	shutReq := make(chan error, 1)

	go func() {
		select {
		case err := <-shutReq:
			cancel(err)
		case <-sig:
			cancel(nil)
		case <-ctx.Done():
		}
	}()

	srv, err := server.New(ctx, o.cfg)
	if err != nil {
		return err
	}

	return srv.Go(ctx, shutReq)
}
