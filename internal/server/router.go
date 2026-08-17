package server

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/raddigo/raddigo/internal/handler"
	"github.com/raddigo/raddigo/internal/middleware"
)

// NewRouter builds the application's Gin engine with middleware and routes.
func NewRouter(
	logger *slog.Logger,
	health *handler.HealthHandler,
	auth *handler.AuthHandler,
	partner *handler.PartnerHandler,
	address *handler.AddressHandler,
	rating *handler.RatingHandler,
) http.Handler {
	gin.SetMode(gin.DebugMode)

	router := gin.New()
	router.Use(middleware.Recoverer(logger), middleware.Logger(logger))

	router.GET("/healthz", health.Health)

	v1 := router.Group("/api/v1")
	{
		authGroup := v1.Group("/auth")
		{
			authGroup.POST("/user/register", auth.Register)
			authGroup.GET("/user/verify", auth.Verify)

			authGroup.POST("/partner/register", partner.Register)
			authGroup.GET("/partner/verify", partner.Verify)
		}

		users := v1.Group("/user")
		{
			users.GET("/partner/search", partner.Search)

			users.POST("/:userId/addresses", address.Create)
			users.GET("/:userId/addresses", address.List)
			users.GET("/:userId/addresses/:addressId", address.Get)
			users.PUT("/:userId/addresses/:addressId", address.Update)
			users.DELETE("/:userId/addresses/:addressId", address.Delete)

			users.POST("/:userId/partner/:partnerId/rating", rating.RatePartner)
			users.GET("/:userId/ratings", rating.ListForUser)
			users.GET("/:userId/rating-summary", rating.SummaryForUser)
		}

		partners := v1.Group("/partner")
		{
			partners.POST("/:partnerId/user/:userId/rating", rating.RateUser)
			partners.GET("/:partnerId/ratings", rating.ListForPartner)
			partners.GET("/:partnerId/rating-summary", rating.SummaryForPartner)
		}
	}

	return router
}
