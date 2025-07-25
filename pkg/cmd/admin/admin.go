package admin

import (
	"dynatron.me/x/stillbox/pkg/config"

	"github.com/urfave/cli/v3"
)

// AdminCommand is the admin command.
func AdminCommand(cfg *config.Configuration) *cli.Command {
	userCmd := &cli.Command{
		Name:    "admin",
		Aliases: []string{"a"},
		Usage:   "administers stillbox",
		Commands: []*cli.Command{
			UsersCommand(cfg),
			DatabaseCommand(cfg),
			CallsCommand(cfg),
		},
	}

	return userCmd
}
