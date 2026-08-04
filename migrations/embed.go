package migrations

import "embed"

// Files contains the versioned SQL migrations shipped with the daemon.
//
//go:embed *.sql
var Files embed.FS
