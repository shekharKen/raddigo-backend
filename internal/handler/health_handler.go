package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthHandler serves liveness/readiness checks.
type HealthHandler struct{}

// NewHealthHandler creates a HealthHandler.
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// Health responds with a simple status payload.
func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
