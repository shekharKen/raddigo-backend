package server

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/raddigo/raddigo/internal/handler"
	"github.com/raddigo/raddigo/internal/middleware"
)

// NewRouter builds the application's Gin engine with middleware and routes.
func NewRouter(logger *slog.Logger, health *handler.HealthHandler, auth *handler.AuthHandler) http.Handler {
	gin.SetMode(gin.DebugMode)

	router := gin.New()
	router.Use(middleware.Recoverer(logger), middleware.Logger(logger))

	router.GET("/healthz", health.Health)

	v1 := router.Group("/api/v1")
	{
		v1.POST("/auth/register", auth.Register)
		v1.GET("/auth/verify", auth.Verify)
	}

	return router
}
