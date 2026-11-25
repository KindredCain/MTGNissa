// Package ui serves the embedded frontend independently of the business API.
package ui

import (
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// Handler returns the embedded frontend handler, or nil for backend-only builds.
// Build with the embed_ui tag after preparing dist to include the frontend.
func Handler() http.Handler {
	files := assets()
	if files == nil {
		return nil
	}
	return newHandler(files)
}

func newHandler(files fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		name := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/"), "/")
		if name == "" {
			name = "index.html"
		}
		if !fs.ValidPath(name) || strings.HasPrefix(name, ".") || strings.Contains(name, "/.") {
			http.NotFound(w, r)
			return
		}
		info, err := fs.Stat(files, name)
		if err != nil {
			// Missing assets must stay 404; HTML here would hide stale bundle URLs.
			if path.Ext(name) != "" || name == "assets" || strings.HasPrefix(name, "assets/") {
				http.NotFound(w, r)
				return
			}
			name = "index.html"
		} else if info.IsDir() {
			http.NotFound(w, r)
			return
		}
		file, err := files.Open(name)
		if err != nil {
			http.Error(w, "frontend is unavailable", http.StatusInternalServerError)
			return
		}
		defer file.Close()
		info, err = file.Stat()
		if err != nil {
			http.Error(w, "frontend is unavailable", http.StatusInternalServerError)
			return
		}
		content, ok := file.(io.ReadSeeker)
		if !ok {
			http.Error(w, "frontend is unavailable", http.StatusInternalServerError)
			return
		}

		// ServeContent preserves deep-link URLs and handles HEAD and byte ranges.
		http.ServeContent(w, r, info.Name(), info.ModTime(), content)
	})
}
