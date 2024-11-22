package client

import (
	"embed"
)

const Prefix = "admin/dist/admin/browser"

//go:embed admin/dist/admin/browser
var Client embed.FS
