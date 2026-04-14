// Package migrations embeds the SQL migration files for use by the db package.
package migrations

import "embed"

// FS holds all *.sql migration files embedded at compile time.
//
//go:embed *.sql
var FS embed.FS
