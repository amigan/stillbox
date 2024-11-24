package config

import (
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func (c *Config) PreRunE() func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		return c.ReadConfig()
	}
}

func New(rootCommand *cobra.Command) *Config {
	c := &Config{}

	rootCommand.PersistentFlags().StringVarP(&c.configPath, "config", "c", "config.yaml", "configuration file")

	return c
}

func (c *Config) ReadConfig() error {
	cfgBytes, err := os.ReadFile(c.configPath)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(cfgBytes, c)
	if err != nil {
		return err
	}

	log.Info().Str("configPath", c.configPath).Msg("read config")

	return nil
}
