package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestUIRoutingBoundary(t *testing.T) {
	frontend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "index.html", time.Time{}, strings.NewReader("<!doctype html>frontend"))
	})
	router := New(Dependencies{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), UI: frontend})
	for _, target := range []string{"/", "/sets", "/decks/12/edit", "/apix", "/healthcare"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
		if response.Code != http.StatusOK || response.Body.String() != "<!doctype html>frontend" {
			t.Fatalf("frontend %s = %d %s", target, response.Code, response.Body.String())
		}
		if response.Header().Get("X-Request-ID") == "" {
			t.Fatal("frontend bypassed global request middleware")
		}
	}
	tests := []struct {
		method string
		target string
		status int
	}{
		{http.MethodGet, "/api", http.StatusNotFound},
		{http.MethodGet, "/api/missing", http.StatusNotFound},
		{http.MethodGet, "/api/v1/missing", http.StatusNotFound},
		{http.MethodPost, "/api/v1/missing", http.StatusNotFound},
		{http.MethodGet, "/health", http.StatusNotFound},
		{http.MethodGet, "/health/missing", http.StatusNotFound},
		{http.MethodPut, "/api/v1/app", http.StatusMethodNotAllowed},
		{http.MethodGet, "/api/v1/app", http.StatusOK},
		{http.MethodGet, "/health/live", http.StatusOK},
	}
	for _, test := range tests {
		t.Run(test.method+test.target, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(test.method, test.target, nil))
			var envelope struct {
				Status    int    `json:"status"`
				RequestID string `json:"request_id"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("API or health request reached frontend: %s", response.Body.String())
			}
			if response.Code != test.status || envelope.Status != test.status || envelope.RequestID == "" {
				t.Fatalf("response = %d %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestBackendOnlyHasNoUIFallback(t *testing.T) {
	router := New(Dependencies{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sets", nil))
	if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), `"code":"not_found"`) {
		t.Fatalf("backend-only response = %d %s", response.Code, response.Body.String())
	}
}
