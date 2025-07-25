package admin

import (
	"context"
	"fmt"

	"dynatron.me/x/stillbox/pkg/calls/callstore"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/rest/client"
	"github.com/urfave/cli/v3"
)

func CallsCommand(cfg *config.Configuration) *cli.Command {
	c := &cfg.Config
	callsCmd := &cli.Command{
		Name:  "calls",
		Usage: "administers calls",
		Commands: []*cli.Command{
			moveCommand(c),
		},
	}

	return callsCmd
}

func moveCommand(cfg *config.Config) *cli.Command {
	c := &cli.Command{
		Name:        "move",
		Aliases:     []string{"mv"},
		Usage:       "moves calls between storage backends",
		Description: "checks partition interval matches whatever is specified in the config",
		UsageText:   "stillbox admin database partitioning",
		Flags:       []cli.Flag{},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cfg.Server.AdminSocket == nil {
				return fmt.Errorf("no admin socket configured")
			}

			params := callstore.MoveCallParams{
				DryRun: true,
			}

			progressCb := func(msg client.ProgressMsg) {
			}

			c, err := client.New(client.UnixSocket(*cfg.Server.AdminSocket))
			if err != nil {
				return err
			}

			err = c.MoveCalls(ctx, params, progressCb)
			if err != nil {
				return err
			}

			return nil
		},
	}

	return c
}
