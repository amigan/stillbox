package ruletime

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// RuleTime specifies a start and end of a time period.
type RuleTime struct {
	Hour   uint8
	Minute uint8
	Second uint8

	Duration time.Duration
}

type coversOptions struct {
	loc *time.Location
}

type CoversOption func(*coversOptions)

// WithLocation makes Covers use the provided *time.Location
func WithLocation(loc *time.Location) CoversOption {
	return func(o *coversOptions) {
		o.loc = loc
	}
}

// Covers returns whether the RuleTime covers the provided time.Time.
func (rt *RuleTime) Covers(t time.Time, opts ...CoversOption) bool {
	o := coversOptions{}
	for _, opt := range opts {
		opt(&o)
	}

	if o.loc == nil {
		o.loc = time.Local
	}

	start := time.Date(t.Year(), t.Month(), t.Day(), int(rt.Hour), int(rt.Minute), int(rt.Second), 0, o.loc)
	end := start.Add(rt.Duration)

	// wrap things around the clock
	if end.Day() > start.Day() || end.Month() > start.Month() || end.Year() > start.Year() {
		start = start.Add(-24 * time.Hour)
		end = start.Add(rt.Duration)
		if t.Sub(start) > 24*time.Hour {
			t = t.Add(-24 * time.Hour)
		}
	}

	if t.After(start) && t.Before(end) {
		return true
	}

	return false
}

// CoversNow returns whether the RuleTime covers this instant.
func (rt *RuleTime) CoversNow(opts ...CoversOption) bool {
	return rt.Covers(time.Now(), opts...)
}

func (rt *RuleTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(rt.String())
}

func (rt *RuleTime) UnmarshalJSON(b []byte) error {
	var s string
	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}

	return rt.Parse(s)
}

func (rt *RuleTime) String() string {
	return fmt.Sprintf("%d:%d:%d+%s", rt.Hour, rt.Minute, rt.Second, rt.Duration.String())
}

// ParseRuleTime emits a RuleTime from a string in the format of `[h]h:[m]m[:[s]s]+duration`
// such as: `08:09:00+1h2m`. Time must be in 24 hour format.
func (rt *RuleTime) Parse(s string) error {
	perr := func(e string) error {
		return fmt.Errorf("parse rule time '%s': %s", s, e)
	}

	werr := func(w error) error {
		return fmt.Errorf("parse rule time '%s': %w", s, w)
	}

	timeDurationParts := strings.Split(s, "+")
	if len(timeDurationParts) != 2 {
		return perr("needs time and duration part")
	}

	timeSpec := strings.Split(timeDurationParts[0], ":")
	if len(timeSpec) != 2 && len(timeSpec) != 3 {
		return perr("invalid time part")
	}

	for i, v := range timeSpec {
		nv, err := strconv.Atoi(v)
		if err != nil {
			return werr(err)
		}

		switch i {
		case 0:
			if nv > 23 || nv < 0 {
				return perr("invalid hours")
			}
			rt.Hour = uint8(nv)
		case 1:
			if nv > 59 || nv < 0 {
				return perr("invalid minutes")
			}
			rt.Minute = uint8(nv)
		case 2:
			if nv > 59 || nv < 0 {
				return perr("invalid seconds")
			}
			rt.Second = uint8(nv)
		}
	}

	dur, err := time.ParseDuration(timeDurationParts[1])
	if err != nil {
		return werr(err)
	}

	if dur < 0 {
		return perr("duration must be positive")
	}

	if dur > 24*time.Hour {
		return perr("duration too long")
	}

	rt.Duration = dur

	return nil
}

// New creates a new RuleTime with the provided value.
func New(s string) (RuleTime, error) {
	rt := RuleTime{}

	return rt, rt.Parse(s)
}
