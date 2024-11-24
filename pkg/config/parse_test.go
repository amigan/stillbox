package config

import (
	"testing"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/jsontypes"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/assert"
)

var expCfg = &Config{
	DB: DB{
		Connect: "postgres://stillbox:somepassword@stillbox:5432/stillbox?sslmode=disable",
		LogQueries: true,
	},
	CORS: CORS{
		AllowedOrigins: []string{
			"http://localhost:*",
},
	},
	Auth: Auth{
		JWTSecret: "somesecret",
		Domain: "xenon",
		AllowInsecure: map[string]bool{
			"localhost": true,
			"stillbox": true,
		},
	},
	Alerting: Alerting{
		Enable: true,
		LookbackDays: 7,
		HalfLife: jsontypes.Duration(30*time.Minute),
		Recent: jsontypes.Duration(2*time.Hour),
		AlertThreshold: 0.3,
		Renotify: common.PtrTo(jsontypes.Duration(30*time.Minute)),
	},
	Log: []Logger{
		Logger{
			Level: common.PtrTo("debug"),
		},
		Logger{
			Level: common.PtrTo("error"),
			File: common.PtrTo("error.log"),
		},
	},
	Listen: ":3051",
	Public: true,
	RateLimit: RateLimit{
		Enable: true,
		Requests: 200,
		Over: 2*time.Minute,
	},
	Notify: Notify{
		NotifyService{
			Provider: "slackwebhook",
			Config: map[string]interface{}{
				"webhookURL": "https://hook",
			},
		},
	},
	Relay: []Relay{
		Relay{
			URL: "http://relay",
			APIKey: "secret",
			Required: true,
		},
	},

}

func TestConfigParse(t *testing.T) {
	c := &Configuration{configPath: "testdata/testconfig.yaml"}

	err := c.read()
	require.NoError(t, err)

	assert.Equal(t, expCfg, &c.Config)
}
