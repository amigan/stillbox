package config

import (
	"fmt"
	"strings"

	"dynatron.me/x/stillbox/internal/common"

	"github.com/go-viper/mapstructure/v2"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

func New(configFile *string) *Configuration {
	if configFile == nil {
		panic("configFile must not be nil")
	}

	return &Configuration{configPath: configFile}
}

func (c *Configuration) Before(ctx *cli.Context) error {
	return c.ReadConfig()
}

func (c *Configuration) ReadConfig() error {
	log.Info().Str("configPath", *c.configPath).Msg("read config")

	return c.read()
}

func (c *Configuration) read() error {
	k := koanf.New(".")
	err := k.Load(file.Provider(*c.configPath), yaml.Parser())
	if err != nil {
		return err
	}

	err = k.Load(env.Provider(common.EnvPrefix, ".", func(s string) string {
		return strings.Replace(strings.ToLower(
			strings.TrimPrefix(s, common.EnvPrefix)), "_", ".", -1)
	}), nil)
	if err != nil {
		return err
	}

	err = k.UnmarshalWithConf("", &c.Config,
		koanf.UnmarshalConf{
			Tag: "yaml",
			DecoderConfig: &mapstructure.DecoderConfig{
				Result:           &c.Config,
				WeaklyTypedInput: true,
				DecodeHook: mapstructure.ComposeDecodeHookFunc(
					mapstructure.StringToTimeDurationHookFunc(),
					mapstructure.TextUnmarshallerHookFunc(),
				),
			},
		})

	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	return nil
}
