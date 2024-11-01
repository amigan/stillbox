package ruletime_test

import (
	"errors"
	"testing"
	"time"

	"dynatron.me/x/stillbox/internal/ruletime"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name      string
		ruleTime  string
		compare   ruletime.RuleTime
		expectErr error
	}{
		{
			name:     "base case",
			ruleTime: "23:45:01+2h1m",
			compare: ruletime.RuleTime{
				23, 45, 1, (2 * time.Hour) + time.Minute,
			},
		},
		{
			name:     "no seconds",
			ruleTime: "23:45+2h1m",
			compare: ruletime.RuleTime{
				23, 45, 0, (2 * time.Hour) + time.Minute,
			},
		},
		{
			name:      "bad hour",
			ruleTime:  "29:45+2h1m",
			expectErr: errors.New("invalid hours"),
		},
		{
			name:      "bad minute",
			ruleTime:  "22:70+2h1m",
			expectErr: errors.New("invalid minutes"),
		},
		{
			name:      "bad minute 2",
			ruleTime:  "22:-70+2h1m",
			expectErr: errors.New("invalid minutes"),
		},
		{
			name:      "bad hour 2",
			ruleTime:  "-20:34+2h1m",
			expectErr: errors.New("invalid hours"),
		},
		{
			name:      "bad seconds",
			ruleTime:  "20:34:94+2h1m",
			expectErr: errors.New("invalid seconds"),
		},
		{
			name:      "negative duration",
			ruleTime:  "20:34:33+-2h1m",
			expectErr: errors.New("duration must be positive"),
		},
		{
			name:      "bad duration",
			ruleTime:  "20:34:04+2j",
			expectErr: errors.New("in duration"),
		},
		{
			name:      "duration too long",
			ruleTime:  "20:34:04+25h",
			expectErr: errors.New("duration too long"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rt, err := ruletime.New(tc.ruleTime)
			if tc.expectErr != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectErr.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.compare, rt)
			}
		})
	}
}

func TestCovers(t *testing.T) {
	tM := func(s string) func(*time.Location) time.Time {
		return func(loc *time.Location) time.Time {
			tm, err := time.ParseInLocation("2006-01-02 15:04:05", "2024-11-01 "+s, loc)
			if err != nil {
				panic(err)
			}

			return tm
		}
	}

	tz := func(s string) *time.Location {
		l, err := time.LoadLocation(s)
		if err != nil {
			panic(err)
		}

		return l
	}

	tests := []struct {
		name     string
		timespec string
		t        func(*time.Location) time.Time
		opts     []ruletime.CoversOption
		loc      *time.Location
		covers   bool
	}{
		{
			name:     "base",
			timespec: "8:45+1h",
			t:        tM("9:00:00"),
			covers:   true,
		},
		{
			name:     "base false",
			timespec: "8:45+1h",
			t:        tM("9:47:00"),
			covers:   false,
		},
		{
			name:     "wrong tz",
			timespec: "8:45+1h",
			t:        tM("9:37:00"),
			loc:      tz("America/New_York"),
			covers:   false,
			opts:     []ruletime.CoversOption{ruletime.WithLocation(time.UTC)},
		},
		{
			name:     "wrong tz 2",
			timespec: "8:45+1h",
			t:        tM("9:07:00"),
			loc:      tz("America/Chicago"),
			covers:   false,
			opts:     []ruletime.CoversOption{ruletime.WithLocation(tz("America/New_York"))},
		},
		{
			name:     "right tz",
			timespec: "8:45+1h",
			t:        tM("9:17:00"),
			loc:      tz("America/New_York"),
			covers:   true,
			opts:     []ruletime.CoversOption{ruletime.WithLocation(tz("America/New_York"))},
		},
		{
			name:     "past midnight",
			timespec: "23:45+1h",
			t:        tM("0:07:00"),
			loc:      tz("America/Chicago"),
			covers:   true,
			opts:     []ruletime.CoversOption{ruletime.WithLocation(tz("America/Chicago"))},
		},
		{
			name:     "not past midnight",
			timespec: "15:00+10h",
			t:        tM("3:07:00"),
			loc:      tz("America/Chicago"),
			covers:   false,
			opts:     []ruletime.CoversOption{ruletime.WithLocation(tz("America/Chicago"))},
		},
		{
			name:     "not past midnight 2",
			timespec: "15:00+10h",
			t:        tM("14:07:00"),
			loc:      tz("America/Chicago"),
			covers:   false,
			opts:     []ruletime.CoversOption{ruletime.WithLocation(tz("America/Chicago"))},
		},
		{
			name:     "not past midnight 3",
			timespec: "15:00+10h",
			t:        tM("15:07:00"),
			loc:      tz("America/Chicago"),
			covers:   true,
			opts:     []ruletime.CoversOption{ruletime.WithLocation(tz("America/Chicago"))},
		},
		{
			name:     "24h duration",
			timespec: "15:00+24h",
			t:        tM("3:07:00"),
			loc:      tz("America/Chicago"),
			covers:   true,
			opts:     []ruletime.CoversOption{ruletime.WithLocation(tz("America/Chicago"))},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rt, err := ruletime.New(tc.timespec)
			require.NoError(t, err)
			loc := time.Local
			if tc.loc != nil {
				loc = tc.loc
			}

			c := rt.Covers(tc.t(loc), tc.opts...)
			if tc.covers {
				assert.True(t, c)
			} else {
				assert.False(t, c)
			}
		})
	}
}
