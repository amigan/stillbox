package config

import (
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type Config struct {
	DB     DB     `yaml:"db"`
	CORS   CORS   `yaml:"cors"`
	Auth   Auth   `yaml:"auth"`
	Listen string `yaml:"listen"`
	Public bool   `yaml:"public"`

	configPath string
}

type Auth struct {
	JWTSecret     string          `yaml:"jwtsecret"`
	Domain        string          `yaml:"domain"`
	AllowInsecure map[string]bool `yaml:"allowInsecureFor"`
}

type CORS struct {
	AllowedOrigins []string `yaml:"allowedOrigins"`
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
