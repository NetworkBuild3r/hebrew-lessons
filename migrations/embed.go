// Package migrations embeds the forward-only SQL for a new database.
//
// 001 is a clean start. Nothing is imported from the previous app.
//
// The SQL lives in this directory, at the repository root, because
// //go:embed paths cannot contain "..". internal/db reads FS.
package migrations

import "embed"

// FS holds the versioned *.sql files. A file name starts with the
// integer version, for example 001_init.sql.
//
//go:embed *.sql
var FS embed.FS
