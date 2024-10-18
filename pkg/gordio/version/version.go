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
