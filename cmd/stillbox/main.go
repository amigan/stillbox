package main

import (
	"context"
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"dynatron.me/x/stillbox/internal/cmd/admin"
	"dynatron.me/x/stillbox/internal/cmd/serve"
	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/version"
	"dynatron.me/x/stillbox/pkg/config"

	"github.com/urfave/cli/v3"
)

const DefaultConfig = "config.yaml"

func main() {
	configFile := DefaultConfig
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: common.TimeFormat})

	cfg := config.New(&configFile)
	app := &cli.Command{
		Name:                   common.AppName,
		Usage:                  "a scanner call server",
		UseShortOptionHandling: true,
		Before:                 cfg.Before,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "config",
				Value:       DefaultConfig,
				Usage:       "configuration file",
				Destination: &configFile,
				Aliases:     []string{"c"},
			},
			&cli.BoolFlag{
				Name:    "version",
				Aliases: []string{"V"},
				Action: func(_ context.Context, _ *cli.Command, v bool) error {
					if v {
						fmt.Print(version.String())
						os.Exit(0)
					}

					return nil
				},
				DefaultText: "",
			},
		},
		Commands: []*cli.Command{
			serve.Command(cfg),
			admin.AdminCommand(cfg),
		},
	}

	err := app.Run(context.Background(), os.Args)
	if err != nil {
		_, _ = os.Stderr.Write([]byte("Error: " + err.Error() + "\n"))
		os.Exit(1)
	}
}
