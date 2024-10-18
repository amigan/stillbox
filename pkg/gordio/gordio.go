package gordio

import (
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
	srv, err := server.New(o.cfg)
	if err != nil {
		return err
	}

	err = srv.Go()
	if err != nil {
		return err
	}

	return nil
}
