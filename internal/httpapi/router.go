package httpapi

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/health"
	"mtgnissa/internal/httpresponse"
)

// New assembles the HTTP router, global middleware, health checks, and API modules.
func New(log *slog.Logger, healthHandler health.Handler, loader cardDataLoader, loadEnabled bool, sets setLister) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.Use(requestID(), accessLog(log), recovery(), bodyLimit(1<<20))

	router.GET("/health/live", healthHandler.Live)
	router.GET("/health/ready", healthHandler.Ready)

	api := router.Group("/api/v1")
	cardDataAPI := api.Group("/card_data")
	cardDataAPI.POST("/load", requireJSONContentType(), rejectUnknownQueryParameters(), startLoad(loader, loadEnabled))
	cardDataAPI.GET("/load/:task_id", rejectUnknownQueryParameters(), getLoad(loader, loadEnabled))
	api.GET("/sets", rejectUnknownQueryParameters("q", "language", "set_type", "sort", "cursor"), getSets(sets))

	registerFallbackRoutes(router)
	return router
}

func registerFallbackRoutes(router *gin.Engine) {
	router.NoMethod(func(c *gin.Context) {
		httpresponse.WriteAppError(c, apperror.New(apperror.CodeMethodNotAllowed, "method not allowed"))
	})
	router.NoRoute(func(c *gin.Context) {
		httpresponse.WriteAppError(c, apperror.New(apperror.CodeNotFound, "route not found"))
	})
}
