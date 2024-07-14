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

	rootCmd := cobra.Command{
		Use: gordio.AppName,
	}

	cfg, err := config.ReadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("Config read failed")
	}

	cmds := []*cobra.Command{gordio.Command(cfg)}
	rootCmd.AddCommand(append(cmds, admin.Command(cfg)...)...)

	err = rootCmd.Execute()
	if err != nil {
		log.Fatal().Err(err).Msg("Dying")
	}

}
