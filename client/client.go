//go:build !noclient

package client

import (
	"embed"
)

const Prefix = "stillbox-ng/dist/stillbox/browser"

//go:embed stillbox-ng/dist/stillbox/browser
var Web embed.FS
