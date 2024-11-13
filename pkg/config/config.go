package config

import (
	"os"
	"sync"
	"time"

	"dynatron.me/x/stillbox/internal/jsontypes"

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
	Notify    Notify    `yaml:"notify"`

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
	Enable         bool                `yaml:"enable" form:"enable"`
	LookbackDays   uint                `yaml:"lookbackDays" form:"lookbackDays"`
	HalfLife       jsontypes.Duration  `yaml:"halfLife" form:"halfLife"`
	Recent         jsontypes.Duration  `yaml:"recent" form:"recent"`
	AlertThreshold float64             `yaml:"alertThreshold" form:"alertThreshold"`
	Renotify       *jsontypes.Duration `yaml:"renotify,omitempty" form:"renotify,omitempty"`
}

type Notify []NotifyService

type NotifyService struct {
	Provider        string                 `yaml:"provider" json:"provider"`
	SubjectTemplate *string                `yaml:"subjectTemplate" json:"subjectTemplate"`
	BodyTemplate    *string                `yaml:"bodyTemplate" json:"bodyTemplate"`
	Config          map[string]interface{} `yaml:"config" json:"config"`
}

func (n *NotifyService) GetS(k, defaultVal string) string {
	if v, has := n.Config[k]; has {
		if v, isString := v.(string); isString {
			return v
		}
		log.Error().Str("configKey", k).Str("provider", n.Provider).Str("default", defaultVal).Msg("notify config value is not a string! using default")
	}

	return defaultVal
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

	log.Info().Str("configPath", c.configPath).Msg("read config")

	return nil
}
