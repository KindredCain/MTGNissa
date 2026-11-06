package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"mtgnissa/internal/httpresponse"
)

// AppConfig contains the public application settings validated during startup.
type AppConfig struct {
	DisplayName string
	TimeZone    string
}

func getApp(app AppConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		httpresponse.WriteJSON(c, http.StatusOK, appResponse{
			DisplayName: app.DisplayName,
			TimeZone:    app.TimeZone,
		})
	}
}
