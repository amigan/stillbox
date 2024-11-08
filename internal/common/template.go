package common

import (
	"errors"
	"fmt"
	"strconv"
	"text/template"
	"time"

	"dynatron.me/x/stillbox/internal/jsontime"
)

var (
	FuncMap = template.FuncMap{
		"f": fmtFloat,
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, errors.New("invalid dict call")
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, errors.New("dict keys must be strings")
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
		"formTime": func(t jsontime.Time) string {
			return time.Time(t).Format("2006-01-02T15:04")
		},
		"ago": func(s string) (string, error) {
			d, err := time.ParseDuration(s)
			if err != nil {
				return "", err
			}
			return time.Now().Add(-d).Format("2006-01-02T15:04"), nil
		},
	}
)

func fmtFloat(v float64, places ...int) string {
	if len(places) > 0 {
		return fmt.Sprintf("%."+strconv.Itoa(places[0])+"f", v)
	}
	return fmt.Sprintf("%.4f", v)
}
