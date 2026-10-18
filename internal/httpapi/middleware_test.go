package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mtgnissa/internal/health"
)

func TestHTTPErrorInterception(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := New(log, health.Handler{}, loaderStub{}, true, setListerStub{})
	tests := []struct {
		name        string
		method      string
		target      string
		body        string
		contentType string
		wantStatus  int
		wantCode    string
	}{
		{
			name:       "unsupported media type",
			method:     http.MethodPost,
			target:     "/api/v1/card_data/load",
			body:       `{"confirmation":"LOAD_CARD_DATA"}`,
			wantStatus: http.StatusUnsupportedMediaType,
			wantCode:   "unsupported_media_type",
		},
		{
			name:       "method not allowed",
			method:     http.MethodPut,
			target:     "/api/v1/card_data/load",
			wantStatus: http.StatusMethodNotAllowed,
			wantCode:   "method_not_allowed",
		},
		{
			name:       "unknown query parameter",
			method:     http.MethodGet,
			target:     "/api/v1/card_data/load/task-1?unexpected=true",
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:        "request too large",
			method:      http.MethodPost,
			target:      "/api/v1/card_data/load",
			body:        `{"confirmation":"` + strings.Repeat("x", 1<<20) + `"}`,
			contentType: "application/json",
			wantStatus:  http.StatusRequestEntityTooLarge,
			wantCode:    "request_too_large",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.target, strings.NewReader(test.body))
			if test.contentType != "" {
				request.Header.Set("Content-Type", test.contentType)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", response.Code, test.wantStatus, response.Body.String())
			}
			if !strings.Contains(response.Body.String(), `"code":"`+test.wantCode+`"`) {
				t.Fatalf("body does not contain error code %q: %s", test.wantCode, response.Body.String())
			}
		})
	}
}
