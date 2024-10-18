package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"dynatron.me/x/stillbox/pkg/gordio"
	"dynatron.me/x/stillbox/pkg/gordio/admin"
	"dynatron.me/x/stillbox/pkg/gordio/config"

	"github.com/spf13/cobra"
)

var (
	Version = "unset"
	Commit  = "unset"
)

func version() {
	fmt.Printf("gordio %s (%s)\nbuilt for %s-%s\n",
		Version, Commit, runtime.GOOS, runtime.GOARCH)
}

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
			version()
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
