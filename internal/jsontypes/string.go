package jsontypes

import (
	"encoding/json"
)

// This type is used when we should unmarshal even values that are valid as another type into a string.
type String string

func (s *String) StringPtr() *string {
	return (*string)(s)
}

func (s String) String() string {
	return string(s)
}

func (s *String) UnmarshalJSON(data []byte) error {
	if n := len(data); n > 1 && data[0] == '"' && data[n-1] == '"' {
		return json.Unmarshal(data, (*string)(s))
	}

	*s = String(data)

	return nil
}
