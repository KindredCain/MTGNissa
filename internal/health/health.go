package health

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"mtgnissa/internal/apperror"
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
