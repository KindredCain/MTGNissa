//go:build embed_ui

package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"mtgnissa/internal/ui"
)

func TestEmbeddedFrontend(t *testing.T) {
	frontend := ui.Handler()
	if frontend == nil {
		t.Fatal("release build has no frontend")
	}
	router := New(Dependencies{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), UI: frontend})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/decks/12/edit", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `id="root"`) {
		t.Fatalf("embedded deep link = %d %s", response.Code, response.Body.String())
	}
	links := regexp.MustCompile(`(?:src|href)="(/assets/[^"]+)"`).FindAllStringSubmatch(response.Body.String(), -1)
	if len(links) == 0 {
		t.Fatal("embedded entry point does not reference production assets")
	}
	for _, link := range links {
		response = httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, link[1], nil))
		if response.Code != http.StatusOK || response.Body.Len() == 0 || strings.Contains(response.Header().Get("Content-Type"), "text/html") {
			t.Fatalf("embedded asset %s = %d %s", link[1], response.Code, response.Header().Get("Content-Type"))
		}
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodHead, "/cards/123", nil))
	if response.Code != http.StatusOK || response.Body.Len() != 0 {
		t.Fatalf("embedded HEAD = %d %q", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/sets", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("embedded unsupported method = %d", response.Code)
	}
}
