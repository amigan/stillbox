package settings

type Defaults map[string]Setting

var ConfigDefaults = Defaults{
	"calls.view.showSourceAlias": false,
}
