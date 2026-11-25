//go:build embed_ui

package ui

import (
	"embed"
	"io/fs"
)

// Explicitly require the entry point so an incomplete release cannot compile.
//
//go:embed dist dist/index.html
var embedded embed.FS

func assets() fs.FS {
	files, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic(err)
	}
	return files
}
