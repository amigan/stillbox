package jsontypes

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/jackc/pgx/v5/pgtype"
	"gopkg.in/yaml.v3"
)

type Time time.Time

func (t *Time) UnmarshalYAML(n *yaml.Node) error {
	var s string

	err := n.Decode(&s)
	if err != nil {
		return err
	}

	tm, err := dateparse.ParseAny(s)
	if err != nil {
		return err
	}
	*t = Time(tm)
	return nil
}

func (t *Time) PGTypeTSTZ() pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{Valid: false}
	}

	return pgtype.Timestamptz{
		Time:  time.Time(*t),
		Valid: true,
	}
}

func (t *Time) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	tm, err := dateparse.ParseAny(s)
	if err != nil {
		return err
	}
	*t = Time(tm)
	return nil
}

func (t Time) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(t))
}

func (t Time) String() string {
	return time.Time(t).String()
}

func (t Time) Time() time.Time {
	return time.Time(t)
}

type Duration time.Duration

func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	var s string

	err := n.Decode(&s)
	if err != nil {
		return err
	}

	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}

	*d = Duration(dur)
	return nil
}

func (d *Duration) UnmarshalText(text []byte) error {
	dur, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}

	*d = Duration(dur)

	return nil
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}

	*d = Duration(dur)
	return nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d))
}

func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

func (d Duration) String() string {
	return time.Duration(d).String()
}

func ParseDuration(s string) (Duration, error) {
	d, err := time.ParseDuration(s)
	return Duration(d), err
}

func ParseAny(s string, opt ...dateparse.ParserOption) (Time, error) {
	t, err := dateparse.ParseAny(s, opt...)
	return Time(t), err
}

func ParseLocal(s string, opt ...dateparse.ParserOption) (Time, error) {
	t, err := dateparse.ParseLocal(s, opt...)
	return Time(t), err
}
