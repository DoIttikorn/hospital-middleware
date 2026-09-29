// Package migrations embeds the SQL migration files so the binary can apply
// them at startup (see database.Migrate).
package migrations

import "embed"

// FS holds the golang-migrate files: NNNNNN_name.up.sql / .down.sql.
//
//go:embed *.sql
var FS embed.FS
