package httpapi

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"mtgnissa/internal/carddata"
	"mtgnissa/internal/health"
	"mtgnissa/internal/httpresponse"
)

type cardDataLoader interface {
	Start() (carddata.Task, error)
	Get(string) (carddata.Task, bool)
}

type loadTaskResponse struct {
	ID         string                 `json:"id"`
	Stage      carddata.Stage         `json:"stage"`
	StartedAt  time.Time              `json:"started_at"`
	FinishedAt *time.Time             `json:"finished_at,omitempty"`
	Error      *loadTaskErrorResponse `json:"error,omitempty"`
}

type loadTaskErrorResponse struct {
	Stage   carddata.Stage `json:"stage"`
	File    string         `json:"file,omitempty"`
	Line    int64          `json:"line,omitempty"`
	Message string         `json:"message"`
}

func newLoadTaskResponse(task carddata.Task) loadTaskResponse {
	response := loadTaskResponse{
		ID:         task.ID,
		Stage:      task.Stage,
		StartedAt:  task.StartedAt,
		FinishedAt: task.FinishedAt,
	}
	if task.Error != nil {
		response.Error = &loadTaskErrorResponse{
			Stage:   task.Error.Stage,
			File:    task.Error.File,
			Line:    task.Error.Line,
			Message: task.Error.Message,
		}
	}
	return response
}

func New(log *slog.Logger, healthHandler health.Handler, loader cardDataLoader, loadEnabled bool) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(requestID(), accessLog(log), recovery(), bodyLimit(1<<20))
	r.GET("/health/live", healthHandler.Live)
	r.GET("/health/ready", healthHandler.Ready)
	api := r.Group("/api/v1/card-data")
	api.POST("/load", startLoad(loader, loadEnabled))
	api.GET("/load/:id", getLoad(loader, loadEnabled))
	r.NoRoute(func(c *gin.Context) {
		httpresponse.WriteError(c, http.StatusNotFound, "not_found", "route not found")
	})
	return r
}

func startLoad(manager cardDataLoader, enabled bool) gin.HandlerFunc {
	type request struct {
		Confirmation string `json:"confirmation"`
	}
	return func(c *gin.Context) {
		if !enabled {
			httpresponse.WriteError(c, http.StatusForbidden, "load_disabled", "card data loading is disabled")
			return
		}
		var req request
		decoder := jsonDecoder(c.Request.Body)
		if err := decoder.Decode(&req); err != nil {
			httpresponse.WriteError(c, http.StatusBadRequest, "invalid_request", "body must be a JSON object containing confirmation")
			return
		}
		if err := ensureEOF(decoder); err != nil {
			httpresponse.WriteError(c, http.StatusBadRequest, "invalid_request", "body must contain one JSON object")
			return
		}
		if req.Confirmation != carddata.Confirmation {
			httpresponse.WriteError(c, http.StatusBadRequest, "confirmation_required", "confirmation must equal "+carddata.Confirmation)
			return
		}
		task, err := manager.Start()
		if errors.Is(err, carddata.ErrRunning) {
			httpresponse.WriteError(c, http.StatusConflict, "load_in_progress", err.Error())
			return
		}
		if err != nil {
			httpresponse.WriteError(c, http.StatusInternalServerError, "internal_error", "could not start card data loading")
			return
		}
		c.Header("Location", "/api/v1/card-data/load/"+task.ID)
		httpresponse.WriteJSON(c, http.StatusAccepted, newLoadTaskResponse(task))
	}
}

func getLoad(manager cardDataLoader, enabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !enabled {
			httpresponse.WriteError(c, http.StatusForbidden, "load_disabled", "card data loading is disabled")
			return
		}
		task, ok := manager.Get(c.Param("id"))
		if !ok {
			httpresponse.WriteError(c, http.StatusNotFound, "task_not_found", "card data load task not found")
			return
		}
		httpresponse.WriteJSON(c, http.StatusOK, newLoadTaskResponse(task))
	}
}

func recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, _ any) {
		httpresponse.WriteError(c, http.StatusInternalServerError, "internal_error", "internal server error")
	})
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
		c.Next()
		log.Info("http request", "method", c.Request.Method, "path", c.Request.URL.Path, "status", c.Writer.Status(), "duration", time.Since(start), "request_id", c.GetString("request_id"))
	}
}
func bodyLimit(bytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, bytes)
		c.Next()
	}
}

type decoderAPI interface {
	Decode(any) error
	DisallowUnknownFields()
}

func jsonDecoder(r io.Reader) decoderAPI {
	d := newJSONDecoder(r)
	d.DisallowUnknownFields()
	return d
}
func ensureEOF(d decoderAPI) error {
	var extra any
	err := d.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("unexpected second JSON value")
	}
	return err
}
