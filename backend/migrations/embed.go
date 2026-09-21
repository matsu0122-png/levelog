// Package migrations embeds the raw SQL migration files so the server binary
// can apply them on startup without needing the source tree present at
// runtime (e.g. inside a minimal Docker image).
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
