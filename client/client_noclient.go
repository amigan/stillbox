//go:build noclient
package client

import (
	"embed"
)

const Prefix = "stillbox/dist/stillbox/browser"

var Client embed.FS
