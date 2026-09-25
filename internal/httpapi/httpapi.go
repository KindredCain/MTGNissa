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
)

type rebuildStarter interface {
	Start() (carddata.Task, error)
	Get(string) (carddata.Task, bool)
}

func New(log *slog.Logger, healthHandler health.Handler, rebuild rebuildStarter, rebuildEnabled bool) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), requestID(), accessLog(log), bodyLimit(1<<20))
	r.GET("/health/live", healthHandler.Live)
	r.GET("/health/ready", healthHandler.Ready)
	api := r.Group("/api/v1/card-data")
	api.POST("/rebuild", startRebuild(rebuild, rebuildEnabled))
	api.GET("/rebuild/:id", getRebuild(rebuild, rebuildEnabled))
	r.NoRoute(func(c *gin.Context) { problem(c, http.StatusNotFound, "not_found", "route not found") })
	return r
}

func startRebuild(manager rebuildStarter, enabled bool) gin.HandlerFunc {
	type request struct {
		Confirmation string `json:"confirmation"`
	}
	return func(c *gin.Context) {
		if !enabled {
			problem(c, http.StatusForbidden, "rebuild_disabled", "card data rebuild is disabled")
			return
		}
		var req request
		decoder := jsonDecoder(c.Request.Body)
		if err := decoder.Decode(&req); err != nil {
			problem(c, 400, "invalid_request", "body must be a JSON object containing confirmation")
			return
		}
		if err := ensureEOF(decoder); err != nil {
			problem(c, 400, "invalid_request", "body must contain one JSON object")
			return
		}
		if req.Confirmation != carddata.Confirmation {
			problem(c, 400, "confirmation_required", "confirmation must equal "+carddata.Confirmation)
			return
		}
		task, err := manager.Start()
		if errors.Is(err, carddata.ErrRunning) {
			problem(c, http.StatusConflict, "rebuild_in_progress", err.Error())
			return
		}
		if err != nil {
			problem(c, 500, "internal_error", "could not start rebuild")
			return
		}
		c.Header("Location", "/api/v1/card-data/rebuild/"+task.ID)
		c.JSON(http.StatusAccepted, task)
	}
}

func getRebuild(manager rebuildStarter, enabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !enabled {
			problem(c, http.StatusForbidden, "rebuild_disabled", "card data rebuild is disabled")
			return
		}
		task, ok := manager.Get(c.Param("id"))
		if !ok {
			problem(c, 404, "task_not_found", "rebuild task not found")
			return
		}
		c.JSON(200, task)
	}
}

func problem(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"code": code, "message": message}, "request_id": c.GetString("request_id")})
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
	return func(c *gin.Context) { c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, bytes); c.Next() }
}

type decoderAPI interface {
	Decode(any) error
	DisallowUnknownFields()
}

func jsonDecoder(r io.Reader) decoderAPI { d := newJSONDecoder(r); d.DisallowUnknownFields(); return d }
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
