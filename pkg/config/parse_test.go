package config

import (
	"testing"
	"time"

	"dynatron.me/x/stillbox/internal/acl"
	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/jsontypes"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var expCfg = &Config{
	Server: Server{
		CORS: CORS{
			AllowedOrigins: []string{
				"http://localhost:*",
			},
		},
		Listen: ":3051",
		Public: true,
		RateLimit: RateLimit{
			Enable:   true,
			Requests: 200,
			Over:     2 * time.Minute,
		},
	},
	DB: DB{
		Connect:    "postgres://stillbox:somepassword@stillbox:5432/stillbox?sslmode=disable",
		LogQueries: true,
	},
	Auth: Auth{
		JWTSecret: "somesecret",
		AllowInsecure: map[string]bool{
			"localhost": true,
			"stillbox":  true,
		},
		APIKeyACL: &acl.IPConfig{
			Deny: []string{
				"6.4.3.2",
			},
			Order: acl.OrderDenyAllow,
		},
	},
	Alerting: Alerting{
		Enable:              true,
		LookbackDays:        7,
		HalfLife:            jsontypes.Duration(30 * time.Minute),
		Recent:              jsontypes.Duration(2 * time.Hour),
		AlertThreshold:      0.3,
		Renotify:            common.PtrTo(jsontypes.Duration(30 * time.Minute)),
		ContextLookback:     jsontypes.Duration(30 * time.Minute),
		CallLengthThreshold: jsontypes.Duration(4 * time.Second),
	},
	Log: []Logger{
		{
			Level: common.PtrTo("debug"),
		},
		{
			Level: common.PtrTo("error"),
			File:  common.PtrTo("error.log"),
		},
	},

	Notify: Notify{
		Backends: []NotifyService{
			{
				Provider: "slackwebhook",
				Config: map[string]any{
					"webhookURL": "https://hook",
				},
			},
		},
	},
	Relay: []Relay{
		{
			URL:      "http://relay",
			APIKey:   "secret",
			Required: true,
		},
	},
}

var expCfg2 = &Config{
	Server: Server{
		CORS: CORS{
			AllowedOrigins: []string{
				"http://localhost:*",
			},
		},
		Listen: ":3051",
		Public: true,
		RateLimit: RateLimit{
			Enable:   false,
			Requests: 500,
			Over:     2 * time.Minute,
		},
	},
	DB: DB{
		Connect:    "postgres://stillbox:somepassword@stillbox:5432/stillbox?sslmode=disable",
		LogQueries: true,
	},
	Auth: Auth{
		JWTSecret: "somesecret",
		AllowInsecure: map[string]bool{
			"localhost": true,
			"stillbox":  true,
		},
	},
	Alerting: Alerting{
		Enable:              true,
		LookbackDays:        7,
		HalfLife:            jsontypes.Duration(30 * time.Minute),
		Recent:              jsontypes.Duration(2 * time.Hour),
		AlertThreshold:      0.3,
		Renotify:            common.PtrTo(jsontypes.Duration(30 * time.Minute)),
		ContextLookback:     jsontypes.Duration(30 * time.Minute),
		CallLengthThreshold: jsontypes.Duration(4 * time.Second),
	},
	Log: []Logger{
		{
			Level: common.PtrTo("debug"),
		},
		{
			Level: common.PtrTo("error"),
			File:  common.PtrTo("error.log"),
		},
	},

	Notify: Notify{
		Backends: []NotifyService{
			{
				Provider: "slackwebhook",
				Config: map[string]any{
					"webhookURL": "https://hook",
				},
			},
		},
	},
	Relay: []Relay{
		{
			URL:      "http://relay",
			APIKey:   "secret",
			Required: true,
		},
	},
}

func TestConfigParse(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		shouldEqual *Config
	}{
		{
			name:        "some defaults",
			filename:    "testconfig.yaml",
			shouldEqual: expCfg,
		},
		{
			name:        "other defaults",
			filename:    "testconfig2.yaml",
			shouldEqual: expCfg2,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &Configuration{configPath: common.PtrTo("testdata/" + tc.filename)}

			err := c.read()
			require.NoError(t, err)

			assert.Equal(t, tc.shouldEqual, &c.Config)
		})
	}
}
