package config

import (
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func (c *Configuration) PreRunE() func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		return c.ReadConfig()
	}
}

func New(rootCommand *cobra.Command) *Configuration {
	c := &Configuration{}

	rootCommand.PersistentFlags().StringVarP(&c.configPath, "config", "c", "config.yaml", "configuration file")

	return c
}

func (c *Configuration) ReadConfig() error {
	log.Info().Str("configPath", c.configPath).Msg("read config")

	return c.read()
}

func (c *Configuration) read() error {
	cfgBytes, err := os.ReadFile(c.configPath)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(cfgBytes, &c.Config)
	if err != nil {
		return err
	}

	return nil
}
