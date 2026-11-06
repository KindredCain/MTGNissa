package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/httpresponse"
)

type capturingReadCloser struct {
	io.Reader
	io.Closer
}

func recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				var cause error
				switch value := recovered.(type) {
				case error:
					cause = value
				default:
					cause = fmt.Errorf("panic: %v", value)
				}
				httpresponse.WriteAppError(c, apperror.Wrap(apperror.CodeInternalError, "internal server error", cause))
			}
		}()
		c.Next()
	}
}

func requestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if id == "" || len(id) > 128 {
			id = time.Now().UTC().Format("20060102T150405.000000000")
		}
		c.Set("request_id", id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

func accessLog(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		requestBody := &bytes.Buffer{}
		if c.Request.Body != nil {
			body := c.Request.Body
			c.Request.Body = &capturingReadCloser{
				Reader: io.TeeReader(body, requestBody),
				Closer: body,
			}
		}
		c.Next()
		if c.Request.Body != nil {
			_, _ = io.Copy(io.Discard, c.Request.Body)
		}
		attributes := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"query_params", c.Request.URL.Query(),
			"request_body", requestBodyLogValue(requestBody.Bytes()),
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", c.GetString("request_id"),
		}
		if last := c.Errors.Last(); last != nil {
			cause := last.Err
			code := apperror.CodeInternalError
			var appErr *apperror.Error
			if errors.As(last.Err, &appErr) {
				code = appErr.Code
				if appErr.Cause != nil {
					cause = appErr.Cause
				}
			}
			attributes = append(attributes, "error_code", code, "error", cause)
			if c.Writer.Status() >= http.StatusInternalServerError || c.Writer.Status() < http.StatusBadRequest {
				log.Error("http request failed", attributes...)
			} else {
				log.Warn("http request rejected", attributes...)
			}
			return
		}
		log.Info("http request", attributes...)
	}
}

func requestBodyLogValue(body []byte) any {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil
	}
	if !json.Valid(body) {
		return string(body)
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return string(body)
	}
	return value
}

func bodyLimit(bytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, bytes)
		c.Next()
	}
}

func requireJSONContentType() gin.HandlerFunc {
	return func(c *gin.Context) {
		mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
		if err != nil || mediaType != "application/json" {
			httpresponse.WriteAppError(c, apperror.Wrap(apperror.CodeUnsupportedMediaType, "Content-Type must be application/json", err))
			return
		}
		c.Next()
	}
}

func rejectUnknownQueryParameters(allowed ...string) gin.HandlerFunc {
	allowedNames := make(map[string]struct{}, len(allowed))
	for _, name := range allowed {
		allowedNames[name] = struct{}{}
	}
	return func(c *gin.Context) {
		for name := range c.Request.URL.Query() {
			if _, ok := allowedNames[name]; ok {
				continue
			}
			httpresponse.WriteAppError(c, apperror.New(apperror.CodeInvalidRequest, "unknown query parameter: "+name))
			return
		}
		c.Next()
	}
}
