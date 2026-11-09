package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRequestTimeoutContextWithoutRequests(t *testing.T) {
	parent := context.Background()
	c := &gin.Context{Request: (&http.Request{}).WithContext(parent)}
	before := time.Now()
	requestTimeout(10 * time.Second)(c)
	deadline, ok := c.Request.Context().Deadline()
	if !ok || deadline.Before(before.Add(10*time.Second)) || deadline.After(time.Now().Add(10*time.Second)) {
		t.Fatalf("request deadline=%v exists=%v", deadline, ok)
	}
	if c.Request.Context().Err() != context.Canceled || parent.Err() != nil {
		t.Fatal("request cleanup failed or canceled the parent context")
	}
	parent, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
	defer cancel()
	c = &gin.Context{Request: (&http.Request{}).WithContext(parent)}
	requestTimeout(10 * time.Second)(c)
	actual, _ := c.Request.Context().Deadline()
	expected, _ := parent.Deadline()
	if !actual.Equal(expected) || parent.Err() != nil {
		t.Fatal("middleware extended or canceled the caller's earlier deadline")
	}
}

func TestRequestBodyLogValuePreservesJSONStructure(t *testing.T) {
	body := []byte(`{"queue":[{"action":"add","card_id":"a"},{"action":"remove","card_id":"b"}]}`)
	encoded, err := json.Marshal(requestBodyLogValue(body))
	if err != nil {
		t.Fatalf("marshal logged request body: %v", err)
	}
	if string(encoded) != string(body) {
		t.Fatalf("logged request body = %s, want %s", encoded, body)
	}
}

func TestHTTPErrorInterception(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := New(Dependencies{Logger: log})
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
