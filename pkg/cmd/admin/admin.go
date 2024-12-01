package admin

import (
	"dynatron.me/x/stillbox/pkg/config"

	"github.com/urfave/cli/v2"
)

// AdminCommand is the admin command.
func AdminCommand(cfg *config.Configuration) *cli.Command {
	userCmd := &cli.Command{
		Name:    "admin",
		Aliases: []string{"a"},
		Usage:   "administers stillbox",
		Subcommands: []*cli.Command{
			UsersCommand(cfg),
			DatabaseCommand(cfg),
		},
	}

	return userCmd
}
