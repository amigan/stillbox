package admin

import (
	"context"

	"dynatron.me/x/stillbox/pkg/authz"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/authz/policy"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/users"

	"github.com/urfave/cli/v3"
)

// AdminCommand is the admin command.
func AdminCommand(cfg *config.Configuration) *cli.Command {
	userCmd := &cli.Command{
		Name:    "admin",
		Aliases: []string{"a"},
		Usage:   "administers stillbox",
		Before: func(ctx context.Context, c *cli.Command) (context.Context, error) {
			rb, err := authz.New(policy.Policy)
			if err != nil {
				return nil, err
			}

			db, err := database.NewClient(ctx, cfg.DB)
			if err != nil {
				return nil, err
			}

			ctx = database.CtxWithDB(ctx, db)
			ctx = authz.CtxWithRBAC(ctx, rb)
			ctx = entities.CtxWithSubject(ctx, entities.NewLocalAdminSubject())
			ctx = users.CtxWithStore(ctx, users.NewStore(db))
			return ctx, nil
		},
		Commands: []*cli.Command{
			UsersCommand(cfg),
			DatabaseCommand(cfg),
			CallsCommand(cfg),
		},
	}

	return userCmd
}
