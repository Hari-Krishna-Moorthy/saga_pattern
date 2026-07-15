// Package migrations embeds the SQL schema files so the service binary
// can apply them on startup without needing the source tree on disk.
package migrations

import "embed"

//go:embed *.sql
var Files embed.FS
