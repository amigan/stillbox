package config

import (
	"gopkg.in/yaml.v3"
	"os"
)

type Config struct {
	DB        DB     `yaml:"db"`
	JWTSecret string `yaml:"jwtsecret"`
	Listen    string `yaml:"listen"`
	Public    bool   `yaml:"public"`
	Domain    string `yaml:"domain"`
}

type DB struct {
	Connect string `yaml:"connect"`
	Driver  string `yaml:"driver"`
}

type ConfigOption func(*configOptions)

type configOptions struct {
	configPath string
}

func WithConfigPath(p string) ConfigOption {
	return func(o *configOptions) {
		o.configPath = p
	}
}

func ReadConfig(opts ...ConfigOption) (*Config, error) {
	o := new(configOptions)

	for _, opt := range opts {
		opt(o)
	}

	cfgBytes, err := os.ReadFile(o.configPath)
	if err != nil {
		return nil, err
	}

	c := new(Config)

	err = yaml.Unmarshal(cfgBytes, c)
	if err != nil {
		return nil, err
	}

	return c, nil
}
