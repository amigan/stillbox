package main

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"dynatron.me/x/stillbox/pkg/gordio"
	"dynatron.me/x/stillbox/pkg/gordio/admin"
	"dynatron.me/x/stillbox/pkg/gordio/config"

	"github.com/spf13/cobra"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	rootCmd := &cobra.Command{
		Use: gordio.AppName,
	}
	cfg := config.New(rootCmd)
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		return cfg.ReadConfig()
	}

	cmds := append([]*cobra.Command{gordio.Command(cfg)}, admin.Command(cfg)...)
	rootCmd.AddCommand(cmds...)

	err := rootCmd.Execute()
	if err != nil {
		log.Fatal().Err(err).Msg("Dying")
	}
}
