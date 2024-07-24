package config

import (
	"github.com/rs/zerolog/log"
	"os"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type Config struct {
	DB        DB     `yaml:"db"`
	JWTSecret string `yaml:"jwtsecret"`
	Listen    string `yaml:"listen"`
	Public    bool   `yaml:"public"`
	Domain    string `yaml:"domain"`

	configPath string
}

type DB struct {
	Connect string `yaml:"connect"`
	Driver  string `yaml:"driver"`
}

func New(cmd *cobra.Command) *Config {
	c := &Config{}
	cmd.PersistentFlags().StringVarP(&c.configPath, "config", "c", "config.yaml", "configuration file")
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

	log.Info().Str("configPath", c.configPath).Msg("read gordio config")

	return nil
}
