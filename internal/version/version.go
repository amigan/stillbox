package version

import (
	"fmt"
	"runtime"

	"dynatron.me/x/stillbox/internal/common"
)

var (
	Name    = common.AppName
	Version = "unset"
	Built   = "unset"
)

func String() string {
	return fmt.Sprintf("%s %s\nbuilt %s for %s-%s\n",
		Name, Version, Built, runtime.GOOS, runtime.GOARCH)
}

func HttpString(app string) string {
	return fmt.Sprintf("stillbox %s/%s (%s/%s)", app, Version, runtime.GOOS, runtime.GOARCH)
}
