package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestFrontendRoutes(t *testing.T) {
	const index = "<!doctype html><html>MTGNissa</html>"
	handler := newHandler(fstest.MapFS{
		"index.html":        {Data: []byte(index)},
		"assets/app.js":     {Data: []byte("export default 42;")},
		"assets/style.css":  {Data: []byte("body { margin: 0; }")},
		"favicon.svg":       {Data: []byte("<svg></svg>")},
		".env":              {Data: []byte("not public")},
		"assets/.hidden.js": {Data: []byte("not public")},
	})
	tests := []struct {
		name        string
		method      string
		target      string
		status      int
		body        string
		contentType string
	}{
		{"home", http.MethodGet, "/", http.StatusOK, index, "text/html"},
		{"entry", http.MethodGet, "/index.html", http.StatusOK, index, "text/html"},
		{"details", http.MethodGet, "/cards/123?language=en", http.StatusOK, index, "text/html"},
		{"editor", http.MethodGet, "/decks/12/edit", http.StatusOK, index, "text/html"},
		{"trailing slash", http.MethodGet, "/sets/", http.StatusOK, index, "text/html"},
		{"unknown page", http.MethodGet, "/new-page", http.StatusOK, index, "text/html"},
		{"javascript", http.MethodGet, "/assets/app.js", http.StatusOK, "export default 42;", "javascript"},
		{"stylesheet", http.MethodGet, "/assets/style.css", http.StatusOK, "body { margin: 0; }", "text/css"},
		{"public file", http.MethodGet, "/favicon.svg", http.StatusOK, "<svg></svg>", "image/svg+xml"},
		{"head page", http.MethodHead, "/decks/12/edit", http.StatusOK, "", "text/html"},
		{"head file", http.MethodHead, "/assets/app.js", http.StatusOK, "", "javascript"},
		{"missing script", http.MethodGet, "/assets/missing.js", http.StatusNotFound, "", "text/plain"},
		{"missing extensionless asset", http.MethodGet, "/assets/missing", http.StatusNotFound, "", "text/plain"},
		{"missing public file", http.MethodGet, "/missing.svg", http.StatusNotFound, "", "text/plain"},
		{"directory", http.MethodGet, "/assets/", http.StatusNotFound, "", "text/plain"},
		{"hidden file", http.MethodGet, "/.env", http.StatusNotFound, "", "text/plain"},
		{"nested hidden file", http.MethodGet, "/assets/.hidden.js", http.StatusNotFound, "", "text/plain"},
		{"traversal", http.MethodGet, "/assets/../index.html", http.StatusNotFound, "", "text/plain"},
		{"unsupported method", http.MethodPost, "/sets", http.StatusMethodNotAllowed, "", "text/plain"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(test.method, test.target, nil))
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d: %s", response.Code, test.status, response.Body.String())
			}
			if test.status == http.StatusOK && response.Body.String() != test.body {
				t.Fatalf("body = %q, want %q", response.Body.String(), test.body)
			}
			if !strings.Contains(response.Header().Get("Content-Type"), test.contentType) {
				t.Fatalf("content type = %q, want %q", response.Header().Get("Content-Type"), test.contentType)
			}
			if response.Header().Get("Location") != "" {
				t.Fatal("frontend request was redirected")
			}
			if response.Header().Get("Cache-Control") != "no-cache" {
				t.Fatal("frontend must revalidate cached content after a deployment")
			}
			if test.status == http.StatusMethodNotAllowed && response.Header().Get("Allow") != "GET, HEAD" {
				t.Fatal("unsupported frontend methods must advertise GET and HEAD")
			}
		})
	}
}

func TestFrontendRangeAndMissingEntry(t *testing.T) {
	handler := newHandler(fstest.MapFS{"assets/app.js": {Data: []byte("0123456789")}})
	request := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	request.Header.Set("Range", "bytes=2-4")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusPartialContent || response.Body.String() != "234" {
		t.Fatalf("range response = %d %q", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sets", nil))
	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "index.html") {
		t.Fatalf("missing entry must fail without leaking filesystem details: %d %s", response.Code, response.Body.String())
	}
}
