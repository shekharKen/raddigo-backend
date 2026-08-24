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
	profile *handler.ProfileHandler,
	authenticate gin.HandlerFunc,
	publicDir string,
) http.Handler {
	gin.SetMode(gin.DebugMode)

	router := gin.New()
	router.Use(middleware.Recoverer(logger), middleware.Logger(logger))

	router.GET("/healthz", health.Health)

	// Serve uploaded assets (e.g. profile images) from the public directory.
	router.Static("/public", publicDir)

	v1 := router.Group("/api/v1")
	{
		authGroup := v1.Group("/auth")
		{
			authGroup.POST("/user/register", auth.Register)
			authGroup.GET("/user/verify", auth.Verify)
			authGroup.POST("/user/login", auth.Login)
			authGroup.POST("/user/forgot-password", auth.ForgotPassword)
			authGroup.POST("/user/reset-password", auth.ResetPassword)

			authGroup.POST("/partner/register", partner.Register)
			authGroup.GET("/partner/verify", partner.Verify)
			authGroup.POST("/partner/login", partner.Login)
			authGroup.POST("/partner/forgot-password", partner.ForgotPassword)
			authGroup.POST("/partner/reset-password", partner.ResetPassword)

			// Serves users and partners: the role is read from the refresh token.
			authGroup.POST("/refresh", auth.Refresh)
		}

		users := v1.Group("/user")
		users.Use(authenticate)
		{
			users.GET("/partner/search", partner.Search)

			users.GET("/:userId/profile", middleware.RequireUser("userId"), profile.GetUserProfile)
			users.PUT("/:userId/profile", middleware.RequireUser("userId"), profile.UpdateUserProfile)
			users.POST("/:userId/profile/image", middleware.RequireUser("userId"), profile.UploadUserImage)

			users.POST("/:userId/addresses", middleware.RequireUser("userId"), address.Create)
			users.GET("/:userId/addresses", middleware.RequireUser("userId"), address.List)
			users.GET("/:userId/addresses/:addressId", middleware.RequireUser("userId"), address.Get)
			users.PUT("/:userId/addresses/:addressId", middleware.RequireUser("userId"), address.Update)
			users.DELETE("/:userId/addresses/:addressId", middleware.RequireUser("userId"), address.Delete)

			users.POST("/:userId/partner/:partnerId/rating", middleware.RequireUser("userId"), rating.RatePartner)
			users.GET("/:userId/ratings", middleware.RequireUser("userId"), rating.ListForUser)
			users.GET("/:userId/rating-summary", middleware.RequireUser("userId"), rating.SummaryForUser)
		}

		partners := v1.Group("/partner")
		partners.Use(authenticate)
		{
			partners.GET("/:partnerId/profile", middleware.RequirePartner("partnerId"), profile.GetPartnerProfile)
			partners.PUT("/:partnerId/profile", middleware.RequirePartner("partnerId"), profile.UpdatePartnerProfile)
			partners.POST("/:partnerId/profile/image", middleware.RequirePartner("partnerId"), profile.UploadPartnerImage)

			partners.POST("/:partnerId/user/:userId/rating", middleware.RequirePartner("partnerId"), rating.RateUser)
			partners.GET("/:partnerId/ratings", rating.ListForPartner)
			partners.GET("/:partnerId/rating-summary", rating.SummaryForPartner)
		}
	}

	return router
}
