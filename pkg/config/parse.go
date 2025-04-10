package config

import (
	"fmt"
	"reflect"
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

func New(configFile string) *Configuration {
	return &Configuration{configPath: configFile}
}

func (c *Configuration) Before(ctx *cli.Context) error {
	return c.ReadConfig()
}

func (c *Configuration) ReadConfig() error {
	log.Info().Str("configPath", c.configPath).Msg("read config")

	return c.read()
}

func Defaults(keyTag, defaultTag string) mapstructure.DecodeHookFunc {
	return func(from reflect.Value, to reflect.Value) (any, error) {
		toType := to.Type()

		if toType.Kind() == reflect.Struct {
			for i := range toType.NumField() {
				setDefault(from, toType.Field(i), keyTag, defaultTag)
			}
		}

		return from.Interface(), nil
	}
}

func setDefault(from reflect.Value, to reflect.StructField, keyTag, defaultTag string) {
	if from.Kind() != reflect.Map {
		return
	}

	defVal, ok := to.Tag.Lookup(defaultTag)
	if !ok {
		return
	}

	key := strings.Split(to.Tag.Get(keyTag), ",")[0]

	for _, e := range from.MapKeys() {
		// key set, no value required
		if key == e.String() {
			return
		}
	}

	fromVal := reflect.ValueOf(defVal)
	toType := to.Type
	if toType.Kind() == reflect.Struct {
		fromVal = reflect.ValueOf(map[string]any{})
		for i := range toType.NumField() {
			setDefault(fromVal, toType.Field(i), keyTag, defaultTag)
		}
	}

	from.SetMapIndex(reflect.ValueOf(key), fromVal)

}

func (c *Configuration) read() error {
	k := koanf.New(".")
	err := k.Load(file.Provider(c.configPath), yaml.Parser())
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
					Defaults("yaml", "default"),
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
