package migrations

import "embed"

// App contains all versioned migrations for the writable App DB.
//
//go:embed app/*.sql
var App embed.FS
