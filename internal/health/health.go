package health

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"mtgnissa/internal/httpresponse"
)

type statusResponse struct {
	Status string `json:"status"`
}

// Handler serves liveness and database-backed readiness checks.
type Handler struct{ Card, App *sql.DB }

func (h Handler) Live(c *gin.Context) {
	httpresponse.WriteJSON(c, http.StatusOK, statusResponse{Status: "ok"})
}
func (h Handler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if h.Card.PingContext(ctx) != nil || h.App.PingContext(ctx) != nil {
		httpresponse.WriteError(c, http.StatusServiceUnavailable, "not_ready", "service is not ready")
		return
	}
	httpresponse.WriteJSON(c, http.StatusOK, statusResponse{Status: "ok"})
}
