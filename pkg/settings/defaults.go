package settings

import "encoding/json"

type Defaults map[Setting]Setting

func MustMarshal(s Setting) json.RawMessage {
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
