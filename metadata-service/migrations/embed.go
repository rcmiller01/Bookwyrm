package migrations

import "embed"

// Files contains embedded metadata-service SQL migrations.
//
//go:embed *.up.sql
var Files embed.FS

