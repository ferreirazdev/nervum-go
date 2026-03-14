// Package migrations embeds the SQL migration files into the binary.
// This allows the compiled binary to run migrations without external files.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
