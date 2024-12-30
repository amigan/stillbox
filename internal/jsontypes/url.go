package jsontypes

import (
	"encoding/json"
	"net/url"

	"gopkg.in/yaml.v3"
)

type URL url.URL

func (u *URL) URL() url.URL {
	return url.URL(*u)
}

func (u *URL) UnmarshalJSON(b []byte) error {
	var s string
	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}

	ur, err := url.Parse(s)
	if err != nil {
		return err
	}

	*u = URL(*ur)
	return nil
}

func (u *URL) UnmarshalYAML(n *yaml.Node) error {
	var s string

	err := n.Decode(&s)
	if err != nil {
		return err
	}

	ur, err := url.Parse(s)
	if err != nil {
		return err
	}

	*u = URL(*ur)
	return nil
}

func (u *URL) UnmarshalText(t []byte) error {
	ur, err := url.Parse(string(t))
	if err != nil {
		return err
	}

	*u = URL(*ur)
	return nil
}
