package sql

import (
	"embed"
)

//go:embed postgres/migrations/*.sql
var Migrations embed.FS
