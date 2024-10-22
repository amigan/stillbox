package main

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"dynatron.me/x/stillbox/pkg/gordio"
	"dynatron.me/x/stillbox/pkg/gordio/admin"
	"dynatron.me/x/stillbox/pkg/gordio/config"
	"dynatron.me/x/stillbox/internal/version"

	"github.com/spf13/cobra"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	rootCmd := &cobra.Command{
		Use: gordio.AppName,
	}
	rootCmd.PersistentFlags().BoolP("version", "V", false, "show version")
	cfg := config.New(rootCmd)
	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		v, _ := rootCmd.PersistentFlags().GetBool("version")
		if v {
			fmt.Print(version.String())
			os.Exit(0)
		}
	}

	cmds := append([]*cobra.Command{gordio.Command(cfg)}, admin.Command(cfg)...)
	rootCmd.AddCommand(cmds...)

	err := rootCmd.Execute()
	if err != nil {
		log.Fatal().Err(err).Msg("Dying")
	}
}
