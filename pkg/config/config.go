package config

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"dynatron.me/x/stillbox/internal/acl"
	"dynatron.me/x/stillbox/internal/jsontypes"

	"github.com/rs/zerolog/log"
)

type Configuration struct {
	Config

	configPath *string `yaml:"-"`
}

type Config struct {
	Server        Server        `yaml:"server"`
	DB            DB            `yaml:"db"`
	Auth          Auth          `yaml:"auth"`
	Alerting      Alerting      `yaml:"alerting"`
	Log           []Logger      `yaml:"log"`
	Notify        Notify        `yaml:"notify"`
	CallStorage   []CallStorage `yaml:"callStorage"`
	Relay         []Relay       `yaml:"relay"`
	Transcription Transcription `yaml:"transcription"`
	Metrics       Metrics       `yaml:"metrics"`
}

type Server struct {
	BaseURL    jsontypes.URL `yaml:"baseURL"`
	DumpRoutes bool          `yaml:"dumpRoutes"`
	UseXRealIP bool          `yaml:"useXRealIP"`
	Listen     string        `yaml:"listen" default:":3051"`
	Public     bool          `yaml:"public"`
	RateLimit  RateLimit     `yaml:"rateLimit"`
	CORS       CORS          `yaml:"cors"`
}

type RateLimit struct {
	Enable   bool          `yaml:"enable" default:"true"`
	Requests int           `yaml:"requests" default:"200"`
	Over     time.Duration `yaml:"over" default:"2m"`

	verifyError sync.Once
}

type Auth struct {
	JWTSecret                string          `yaml:"jwtsecret"`
	AllowInsecure            map[string]bool `yaml:"allowInsecureFor"`
	SameSiteNoneWhenInsecure bool            `yaml:"sameSiteNoneForInsecure"`
	APIKeyACL                *acl.IPConfig   `yaml:"apiKeyACL"`
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

type Metrics struct {
	Enabled  bool          `yaml:"enabled"`
	Path     string        `yaml:"path"`
	Username string        `yaml:"username"`
	Password string        `yaml:"password"`
	ACL      *acl.IPConfig `yaml:"acl"`
}

type Alerting struct {
	Enable              bool                `yaml:"enable" form:"enable"`
	LookbackDays        uint                `yaml:"lookbackDays" form:"lookbackDays"`
	HalfLife            jsontypes.Duration  `yaml:"halfLife" form:"halfLife"`
	Recent              jsontypes.Duration  `yaml:"recent" form:"recent"`
	AlertThreshold      float64             `yaml:"alertThreshold" form:"alertThreshold"`
	Renotify            *jsontypes.Duration `yaml:"renotify,omitempty" form:"renotify,omitempty"`
	Transcripts         uint                `yaml:"transcripts" form:"transcripts"`
	MaxContext          uint                `yaml:"maxContext" form:"maxContext"`
	CallLengthThreshold jsontypes.Duration  `yaml:"callLengthThreshold" form:"callLengthThreshold" default:"4s"`
	ContextLookback     jsontypes.Duration  `yaml:"contextLookback" form:"contextLookback" default:"10m"`
}

type Relay struct {
	URL      string `yaml:"url"`
	APIKey   string `yaml:"apiKey"`
	Required bool   `yaml:"required"`
}

type Notify []NotifyService

type NotifyService struct {
	Provider        string    `yaml:"provider" json:"provider"`
	SubjectTemplate *string   `yaml:"subjectTemplate" json:"subjectTemplate"`
	BodyTemplate    *string   `yaml:"bodyTemplate" json:"bodyTemplate"`
	Config          ConfigMap `yaml:"config" json:"config"`
}

type ConfigMap map[string]any

type CallStorage struct {
	Name     string             `yaml:"name"`
	ReadOnly bool               `yaml:"readOnly"`
	Backend  string             `yaml:"backend"`
	Filter   ConfigMap          `yaml:"filter"`
	Config   ConfigMap          `yaml:"config"`
	OnError  StorageDisposition `yaml:"onError"`
}

type StorageDisposition int

const (
	OnErrorNextThenFail StorageDisposition = iota
	OnErrorNextThenDB
	OnErrorFail
	OnErrorDB
)

func (sd *StorageDisposition) UnmarshalText(s []byte) error {
	t := strings.ToLower(string(s))
	d, h := map[string]StorageDisposition{
		"next-then-fail": OnErrorNextThenFail,
		"next-then-db":   OnErrorNextThenDB,
		"fail":           OnErrorFail,
		"db":             OnErrorDB,
	}[t]
	if !h {
		return fmt.Errorf("invalid storage disposition '%s'", t)
	}

	*sd = d

	return nil
}

type Transcription struct {
	Filter         ConfigMap `yaml:"filter,omitempty"`
	AtLeastSeconds int       `yaml:"atLeastSeconds"`
	Workers        []Worker  `yaml:"workers"`
}
type Worker struct {
	Type   string    `yaml:"type"`
	Config ConfigMap `yaml:"config,omitempty"`
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
