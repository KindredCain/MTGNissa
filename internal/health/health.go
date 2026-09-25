package health

import (
	"context"
	"database/sql"
	"time"

	"github.com/gin-gonic/gin"
)

type Handler struct{ Card, App *sql.DB }

func (h Handler) Live(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) }
func (h Handler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if h.Card.PingContext(ctx) != nil || h.App.PingContext(ctx) != nil {
		c.JSON(503, gin.H{"status": "not_ready"})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}
