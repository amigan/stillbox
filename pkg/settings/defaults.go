package settings

import "encoding/json"

type Defaults map[string]Setting

func MustMarshal(s any) json.RawMessage {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}

	return b
}

var ConfigDefaults = Defaults{
	prefsName("stillbox"): MustMarshal(Defaults{
		"calls.view.showSourceAlias": false,
	}),
}
