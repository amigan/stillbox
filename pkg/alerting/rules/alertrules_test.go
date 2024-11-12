package rules_test

import (
	"encoding/json"
	"errors"
	"math"
	"testing"
	"time"

	"dynatron.me/x/stillbox/internal/ruletime"
	"dynatron.me/x/stillbox/internal/trending"
	"dynatron.me/x/stillbox/pkg/alerting/rules"
	"dynatron.me/x/stillbox/pkg/talkgroups"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAlertConfig(t *testing.T) {
	ac := talkgroups.NewAlertConfig()
	parseTests := []struct {
		name      string
		tg        talkgroups.ID
		conf      string
		compare   rules.AlertRules
		expectErr error
	}{
		{
			name: "base case",
			tg:   talkgroups.TG(197, 3),
			conf: `[{"times":["7:00+2h","01:00+1h","16:00+1h","19:00+4h"],"mult":0.2},{"times":["11:00+1h","15:00+30m","16:03+20m"],"mult":2.0}]`,
			compare: rules.AlertRules{
				{
					Times: []ruletime.RuleTime{
						ruletime.Must(ruletime.New("7:00+2h")),
						ruletime.Must(ruletime.New("1:00+1h")),
						ruletime.Must(ruletime.New("16:00+1h")),
						ruletime.Must(ruletime.New("19:00+4h")),
					},
					ScoreMultiplier: 0.2,
				},
				{
					Times: []ruletime.RuleTime{
						ruletime.Must(ruletime.New("11:00+1h")),
						ruletime.Must(ruletime.New("15:00+30m")),
						ruletime.Must(ruletime.New("16:03+20m")),
					},
					ScoreMultiplier: 2.0,
				},
			},
		},
		{
			name:      "bad spec",
			tg:        talkgroups.TG(197, 3),
			conf:      `[{"times":["26:00+2h","01:00+1h","19:00+4h"],"mult":0.2},{"times":["11:00+1h","15:00+30m"],"mult":2.0}]`,
			expectErr: errors.New("'26:00+2h': invalid hours"),
		},
	}

	for _, tc := range parseTests {
		t.Run(tc.name, func(t *testing.T) {
			var ar rules.AlertRules
			err := json.Unmarshal([]byte(tc.conf), &ar)
			if tc.expectErr != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectErr.Error())
			} else {
				ac.Add(tc.tg, ar)
				assert.Equal(t, tc.compare, ac.GetRules(tc.tg))
			}
		})
	}

	tMust := func(s string) time.Time {
		t, err := time.ParseInLocation("2006-01-02 15:04", "2024-11-02 "+s, time.Local)
		if err != nil {
			panic(err)
		}

		return t
	}

	evalTests := []struct {
		name        string
		tg          talkgroups.ID
		t           time.Time
		origScore   float64
		expectScore float64
	}{
		{
			name:        "base eval",
			tg:          talkgroups.TG(197, 3),
			t:           tMust("1:20"),
			origScore:   3,
			expectScore: 0.6,
		},
		{
			name:        "base eval",
			tg:          talkgroups.TG(197, 3),
			t:           tMust("23:03"),
			origScore:   3,
			expectScore: 3,
		},
		{
			name:        "base eval",
			tg:          talkgroups.TG(197, 3),
			t:           tMust("8:03"),
			origScore:   1.0,
			expectScore: 0.2,
		},
		{
			name:        "base eval",
			tg:          talkgroups.TG(197, 3),
			t:           tMust("15:15"),
			origScore:   3.0,
			expectScore: 6.0,
		},
		{
			name:        "overlapping eval",
			tg:          talkgroups.TG(197, 3),
			t:           tMust("16:10"),
			origScore:   1.0,
			expectScore: 0.4,
		},
	}

	for _, tc := range evalTests {
		t.Run(tc.name, func(t *testing.T) {
			cs := trending.Score[talkgroups.ID]{
				ID:    tc.tg,
				Score: tc.origScore,
			}
			assert.Equal(t, tc.expectScore, toFixed(cs.Score*ac.ApplyAlertRules(cs.ID, tc.t), 5))
		})
	}
}

func round(num float64) int {
	return int(num + math.Copysign(0.5, num))
}

func toFixed(num float64, precision int) float64 {
	output := math.Pow(10, float64(precision))
	return float64(round(num*output)) / output
}
