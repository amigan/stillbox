//go:build noclient

package client

import (
	"embed"
)

const Prefix = "web/dist/stillbox/browser"

var Web embed.FS
