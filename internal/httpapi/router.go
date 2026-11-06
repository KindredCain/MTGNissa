package httpapi

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/health"
	"mtgnissa/internal/httpresponse"
)

// Dependencies contains runtime settings and services used by the HTTP router.
type Dependencies struct {
	Logger      *slog.Logger
	Health      health.Handler
	Loader      cardDataLoader
	LoadEnabled bool
	App         AppConfig
	Sets        setLister
	Tokens      tokenLister
}

// New assembles the HTTP router, global middleware, health checks, and API modules.
func New(deps Dependencies) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.Use(requestID(), accessLog(deps.Logger), recovery(), bodyLimit(1<<20))

	router.GET("/health/live", deps.Health.Live)
	router.GET("/health/ready", deps.Health.Ready)

	api := router.Group("/api/v1")
	api.GET("/app", rejectUnknownQueryParameters(), getApp(deps.App))
	cardDataAPI := api.Group("/card_data")
	cardDataAPI.POST("/load", requireJSONContentType(), rejectUnknownQueryParameters(), startLoad(deps.Loader, deps.LoadEnabled))
	cardDataAPI.GET("/load/:task_id", rejectUnknownQueryParameters(), getLoad(deps.Loader, deps.LoadEnabled))
	api.GET("/sets", rejectUnknownQueryParameters("q", "language", "set_type", "sort", "cursor"), getSets(deps.Sets))
	api.GET("/sets/options", rejectUnknownQueryParameters(), getSetOptions(deps.Sets))
	api.GET("/tokens", rejectUnknownQueryParameters("set_code"), getTokens(deps.Tokens))

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
