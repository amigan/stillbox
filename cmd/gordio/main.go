package main

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"dynatron.me/x/stillbox/pkg/gordio"
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

	rootCmd.AddCommand(gordio.Command(cfg))

	err = rootCmd.Execute()
	if err != nil {
		log.Fatal().Err(err).Msg("Dying")
	}

}
