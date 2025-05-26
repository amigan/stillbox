package config

import (
	"fmt"
	"reflect"
	"strconv"
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

// New creates a new Configuration, but does not read it.
// configFile is a pointer so that it may be mutated by koanf options.
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
	c.Config = Config{} // zero for hup change detection
	log.Info().Str("configPath", *c.configPath).Msg("read config")

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
	var defVal string
	if from.Kind() != reflect.Map {
		return
	}

	fromM := from.Interface().(map[string]any)

	defVal, defaultSet := to.Tag.Lookup(defaultTag)
	key := strings.Split(to.Tag.Get(keyTag), ",")[0]
	if key == "" {
		return
	}

	toKind := to.Type.Kind()

	fromElem, hasFrom := fromM[key]

	var fromVal reflect.Value

	switch toKind {
	case reflect.Struct:
		if !hasFrom {
			fromVal = reflect.ValueOf(map[string]any{})
		} else {
			fromVal = reflect.ValueOf(fromElem)
		}

		for i := range to.Type.NumField() {
			field := to.Type.Field(i)

			if !field.IsExported() {
				continue
			}

			setDefault(fromVal, field, keyTag, defaultTag)
		}
	default:
		if hasFrom || !defaultSet {
			return
		}
		fromVal = reflect.ValueOf(defVal)
		if !hasFrom && toKind != reflect.Struct {
			if dvI, err := strconv.Atoi(defVal); err == nil {
				fromVal = reflect.ValueOf(dvI)
			} else if dvB, err := strconv.ParseBool(defVal); err == nil {
				fromVal = reflect.ValueOf(dvB)
			} else if dvF, err := strconv.ParseFloat(defVal, 64); err == nil {
				fromVal = reflect.ValueOf(dvF)
			}
		}
	}

	if !hasFrom {
		from.SetMapIndex(reflect.ValueOf(key), fromVal)
	}
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
