package config

import (
	"sync"
	"time"

	"dynatron.me/x/stillbox/internal/jsontypes"

	"github.com/rs/zerolog/log"
)

type Configuration struct {
	Config

	configPath *string `yaml:"-"`
}

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
	Relay     []Relay   `yaml:"relay"`
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
	Connect    string    `yaml:"connect"`
	LogQueries bool      `yaml:"logQueries"`
	Partition  Partition `yaml:"partition"`
}

type Partition struct {
	Enabled      bool   `yaml:"enabled"`
	Schema       string `yaml:"schema"`
	Interval     string `yaml:"interval"`
	Retain       int    `yaml:"retain"`
	PreProvision *int   `yaml:"preProvision"`
	Drop         bool   `yaml:"detach"`
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

type Relay struct {
	URL      string `yaml:"url"`
	APIKey   string `yaml:"apiKey"`
	Required bool   `yaml:"required"`
}

type Notify []NotifyService

type NotifyService struct {
	Provider        string                 `yaml:"provider" json:"provider"`
	SubjectTemplate *string                `yaml:"subjectTemplate" json:"subjectTemplate"`
	BodyTemplate    *string                `yaml:"bodyTemplate" json:"bodyTemplate"`
	Config          map[string]interface{} `yaml:"config" json:"config"`
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
