package admin

import (
	"context"
	"errors"
	"fmt"
	"os"

	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/database/partman"

	"github.com/jackc/pgx/v5"
	"github.com/urfave/cli/v2"
)

// DatabaseCommand is the Database command.
func DatabaseCommand(cfg *config.Configuration) *cli.Command {
	c := &cfg.Config
	userCmd := &cli.Command{
		Name:    "database",
		Aliases: []string{"db"},
		Usage:   "administers database",
		Subcommands: []*cli.Command{
			partitioningCommand(c),
		},
	}

	return userCmd
}

func partitioningCommand(cfg *config.Config) *cli.Command {
	c := &cli.Command{
		Name:    "partitioning",
		Aliases: []string{"part"},
		// someday this will say "changes" instead of "checks"
		Usage:       "checks partition interval",
		Description: "checks partition interval matches whatever is specified in the config",
		UsageText:   "stillbox admin database partitioning",
		Args:        true,
		Action: func(cctx *cli.Context) error {
			ctx := context.Background()
			db, err := database.NewClient(ctx, cfg.DB)
			if err != nil {
				return err
			}

			return Repartition(ctx, db, cfg.DB.Partition)
		},
	}

	return c
}

func Repartition(ctx context.Context, db database.Store, cfg config.Partition) error {
	pm, err := partman.New(db, cfg)
	if err != nil {
		return err
	}

	return db.InTx(ctx, func(db database.Store) error {
		parts, err := db.GetTablePartitions(ctx, cfg.Schema, partman.CallsTable)
		if err != nil {
			return err
		}

		exist, err := pm.ExistingPartitions(parts)
		if err != nil {
			return err
		}

		if len(exist) < 1 {
			return errors.New("no partitions found")
		}

		intv := exist[0].Interval
		for _, p := range exist {
			if p.Interval != intv {
				return errors.New("inconsistent partition intervals found")
			}
		}

		if pm.Interval() == intv {
			fmt.Fprintf(os.Stderr, "config interval '%s' and all extant, attached partitions agree; doing nothing\n", string(pm.Interval()))

			return nil
		}

		return nil
	}, pgx.TxOptions{})
}
