package version

import (
	"fmt"
	"runtime"
)

var (
	Name    = "gordio"
	Version = "unset"
	Built   = "unset"
)

func String() string {
	return fmt.Sprintf("gordio %s\nbuilt %s for %s-%s\n",
		Version, Built, runtime.GOOS, runtime.GOARCH)
}

func HttpString(app string) string {
	return fmt.Sprintf("stillbox %s/%s (%s/%s)", app, Version, runtime.GOOS, runtime.GOARCH)
}
