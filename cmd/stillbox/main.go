package main

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/version"
	"dynatron.me/x/stillbox/pkg/cmd/admin"
	"dynatron.me/x/stillbox/pkg/cmd/serve"
	"dynatron.me/x/stillbox/pkg/config"

	"github.com/spf13/cobra"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: common.TimeFormat})

	rootCmd := &cobra.Command{
		Use: common.AppName,
	}
	rootCmd.PersistentFlags().BoolP("version", "V", false, "show version")
	cfg := config.New(rootCmd)
	rootCmd.PreRun = func(cmd *cobra.Command, args []string) {
		v, _ := rootCmd.PersistentFlags().GetBool("version")
		if v {
			fmt.Print(version.String())
			os.Exit(0)
		}
	}

	cmds := append([]*cobra.Command{serve.Command(cfg)}, admin.Command(cfg)...)
	rootCmd.AddCommand(cmds...)

	err := rootCmd.Execute()
	if err != nil {
		log.Fatal().Err(err).Msg("Dying")
	}
}
