//go:build !noclient

package ui

import (
	"embed"
)

const Prefix = "stillbox/dist/stillbox/browser"

//go:embed stillbox/dist/stillbox/browser
var Client embed.FS
