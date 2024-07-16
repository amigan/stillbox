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
	rootCmd.PersistentFlags().StringP("config", "c", "config.yaml", "config file")
	
	err := rootCmd.ParseFlags(os.Args)
	if err != nil {
		log.Fatal().Err(err).Msg("parsing flags")
	}

	cfgPath, err := rootCmd.PersistentFlags().GetString("config")
	if err != nil {
		log.Fatal().Err(err).Msg("failed parsing config path")
	}

	cfg, err := config.ReadConfig(config.WithConfigPath(cfgPath))
	if err != nil {
		log.Fatal().Err(err).Msg("Config read failed")
	}

	cmds := append([]*cobra.Command{gordio.Command(cfg)}, admin.Command(cfg)...)
	rootCmd.AddCommand(cmds...)

	err = rootCmd.Execute()
	if err != nil {
		log.Fatal().Err(err).Msg("Dying")
	}

}
