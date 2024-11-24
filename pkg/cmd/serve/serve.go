package serve

import (
	"context"
	"os"
	"os/signal"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/server"

	"github.com/urfave/cli/v2"
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

func (o *ServeOptions) Options(_ *cli.Context) error {
	return nil
}

func (o *ServeOptions) Execute() error {
	ctx, cancel := context.WithCancel(context.Background())

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer func() {
		signal.Stop(sig)
		cancel()
	}()

	go func() {
		select {
		case <-sig:
			cancel()
		case <-ctx.Done():
		}
	}()

	srv, err := server.New(ctx, o.cfg)
	if err != nil {
		return err
	}

	return srv.Go(ctx)
}
