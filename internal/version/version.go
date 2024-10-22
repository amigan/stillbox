package version

import (
	"fmt"
	"net/http"
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

func UserAgent(hdr http.Header, app string) {
	hdr.Set("User-Agent", fmt.Sprintf("stillbox %s/%s (%s/%s)", app, Version, runtime.GOOS, runtime.GOARCH))
}
