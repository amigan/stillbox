package gordio

import (
	"context"
	"os"
	"os/signal"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/gordio/config"
	"dynatron.me/x/stillbox/pkg/gordio/server"

	"github.com/spf13/cobra"
)

const AppName = "gordio"

type ServeOptions struct {
	cfg *config.Config
}

func Command(cfg *config.Config) *cobra.Command {
	opts := makeOptions(cfg)
	serveCmd := &cobra.Command{
		Use:               "serve",
		Short:             "starts the" + AppName + " server",
		PersistentPreRunE: cfg.PreRunE(),
		RunE:              common.RunE(opts),
	}

	return serveCmd
}

func makeOptions(cfg *config.Config) *ServeOptions {
	return &ServeOptions{
		cfg: cfg,
	}
}

func (o *ServeOptions) Options(_ *cobra.Command, args []string) error {
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
