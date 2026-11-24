package health

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/config"
	"mtgnissa/internal/httpresponse"
)

type statusResponse struct {
	Status string `json:"status"`
}

// Handler serves liveness and database-backed readiness checks.
type Handler struct{ Card, App *sql.DB }

// Live reports HTTP liveness without checking database availability.
func (h Handler) Live(c *gin.Context) {
	httpresponse.WriteJSON(c, http.StatusOK, statusResponse{Status: "ok"})
}

// Ready checks both databases within a shared timeout before reporting readiness.
func (h Handler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), config.HealthCheckTimeout)
	defer cancel()
	if err := h.Card.PingContext(ctx); err != nil {
		httpresponse.WriteAppError(c, apperror.Wrap(apperror.CodeDependencyUnavailable, "card database is unavailable", err))
		return
	}
	if err := h.App.PingContext(ctx); err != nil {
		httpresponse.WriteAppError(c, apperror.Wrap(apperror.CodeDependencyUnavailable, "application database is unavailable", err))
		return
	}
	httpresponse.WriteJSON(c, http.StatusOK, statusResponse{Status: "ok"})
}
