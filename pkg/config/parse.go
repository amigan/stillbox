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
	//	"github.com/knadh/koanf/providers/posflag"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
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
	k := koanf.New(".")
	err := k.Load(file.Provider(c.configPath), yaml.Parser())
	if err != nil {
		return err
	}

	k.Load(env.Provider(common.EnvPrefix, ".", func(s string) string {
		return strings.Replace(strings.ToLower(
			strings.TrimPrefix(s, common.EnvPrefix)), "_", ".", -1)
	}), nil)

	err = k.UnmarshalWithConf("", &c.Config,
		koanf.UnmarshalConf{
			Tag: "yaml",
			DecoderConfig: &mapstructure.DecoderConfig{
				Result: &c.Config,
				WeaklyTypedInput: true,
				DecodeHook: mapstructure.ComposeDecodeHookFunc(
					mapstructure.StringToTimeDurationHookFunc(),
					mapstructure.TextUnmarshallerHookFunc(),
				),
			},
		})

	if err != nil {
		return fmt.Errorf("unmarshal err: %w", err)
	}

	return nil
}
