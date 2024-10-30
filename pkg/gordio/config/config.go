package config

import (
	"os"
	"sync"
	"time"

	"dynatron.me/x/stillbox/internal/jsontime"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type Config struct {
	DB        DB        `yaml:"db"`
	CORS      CORS      `yaml:"cors"`
	Auth      Auth      `yaml:"auth"`
	Alerting  Alerting  `yaml:"alerting"`
	Log       []Logger  `yaml:"log"`
	Listen    string    `yaml:"listen"`
	Public    bool      `yaml:"public"`
	RateLimit RateLimit `yaml:"rateLimit"`

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
}

type Logger struct {
	File  *string `yaml:"file"`
	Level *string `yaml:"level"`
}

type RateLimit struct {
	Enable   bool          `yaml:"enable"`
	Requests int           `yaml:"requests"`
	Over     time.Duration `yaml:"over"`

	verifyError sync.Once
}

type Alerting struct {
	Enable       bool              `yaml:"enable"`
	LookbackDays uint              `yaml:"lookbackDays"`
	HalfLife     jsontime.Duration `yaml:"halfLife"`
	Recent       jsontime.Duration `yaml:"recent"`
}

func (rl *RateLimit) Verify() bool {
	if rl.Enable {
		if rl.Requests > 0 && rl.Over > 0 {
			return true
		}

		rl.verifyError.Do(func() {
			log.Error().Int("requests", rl.Requests).Str("over", rl.Over.String()).Msg("rate limit config makes no sense, disabled")
		})
	}

	return false
}

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

	log.Info().Str("configPath", c.configPath).Msg("read gordio config")

	return nil
}
