// Package migrations holds each part's SQL, in one folder per part.
package migrations

import "embed"

// Files holds every part's migrations.
//
//go:embed */*.sql
var Files embed.FS
