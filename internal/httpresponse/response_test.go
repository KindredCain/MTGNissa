package httpresponse

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"mtgnissa/internal/apperror"
)

func TestRequestDeadlineResponseEnvelopeWithoutRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	for _, test := range []struct {
		name  string
		write func(*gin.Context)
	}{
		{"success after deadline", func(c *gin.Context) {
			WriteJSON(c, http.StatusOK, "late result")
		}},
		{"dependency failure after deadline", func(c *gin.Context) {
			WriteAppError(c, apperror.Wrap(apperror.CodeDependencyUnavailable, "unavailable", context.DeadlineExceeded))
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			// Exercise response helpers directly; no router or HTTP request is executed.
			c.Request = (&http.Request{}).WithContext(ctx)
			c.Set("request_id", "timeout-test")
			test.write(c)
			var body envelope
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if recorder.Code != http.StatusGatewayTimeout || body.Status != http.StatusGatewayTimeout || body.Data != nil || body.Error == nil || body.Error.Code != "request_timeout" || body.RequestID != "timeout-test" || len(c.Errors) != 1 {
				t.Fatalf("response=%+v errors=%v", body, c.Errors)
			}
		})
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = (&http.Request{}).WithContext(context.Background())
	WriteAppError(c, apperror.Wrap(apperror.CodeDependencyUnavailable, "readiness check timed out", context.DeadlineExceeded))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatal("a dependency's shorter deadline was misclassified as a request timeout")
	}
}

func TestWriteAppErrorMapsValidationDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Set("request_id", "request-1")

	WriteAppError(context, apperror.WithDetails(
		apperror.CodeValidationFailed,
		"request validation failed",
		[]apperror.Detail{{Field: "sections[0].cards[0].quantity", Reason: "must_be_positive"}},
	))

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnprocessableEntity)
	}
	var body struct {
		Status    int    `json:"status"`
		Data      any    `json:"data"`
		RequestID string `json:"request_id"`
		Error     struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Details []struct {
				Field  string `json:"field"`
				Reason string `json:"reason"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != http.StatusUnprocessableEntity || body.Data != nil || body.RequestID != "request-1" {
		t.Fatalf("unexpected envelope: %#v", body)
	}
	if body.Error.Code != string(apperror.CodeValidationFailed) || body.Error.Message != "request validation failed" {
		t.Fatalf("unexpected error: %#v", body.Error)
	}
	if len(body.Error.Details) != 1 || body.Error.Details[0].Field != "sections[0].cards[0].quantity" || body.Error.Details[0].Reason != "must_be_positive" {
		t.Fatalf("unexpected details: %#v", body.Error.Details)
	}
	if len(context.Errors) != 1 {
		t.Fatalf("registered errors = %d, want 1", len(context.Errors))
	}
}

func TestWriteAppErrorHidesUnknownErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	WriteAppError(context, errors.New("password=secret database failure"))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if body := recorder.Body.String(); body == "" || strings.Contains(body, "password=secret") {
		t.Fatalf("unsafe response body: %s", body)
	}
}
