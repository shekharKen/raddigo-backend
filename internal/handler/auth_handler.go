package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/raddigo/raddigo/internal/dto"
	"github.com/raddigo/raddigo/internal/model"
	"github.com/raddigo/raddigo/internal/utils"
)

// userService abstracts user registration and verification logic.
type userService interface {
	Register(ctx context.Context, in dto.RegisterRequest) (model.User, error)
	VerifyEmail(ctx context.Context, token string) error
}

// AuthHandler exposes authentication-related HTTP handlers.
type AuthHandler struct {
	svc userService
}

// NewAuthHandler creates an AuthHandler.
func NewAuthHandler(svc userService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// Register handles POST /api/v1/auth/register.
func (h *AuthHandler) Register(c *gin.Context) {
	var in dto.RegisterRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: "invalid request body"})
		return
	}

	user, err := h.svc.Register(c.Request.Context(), in)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "registration successful, please check your email to verify your account",
		"user":    user,
		"token":   user.VerifyToken,
	})
}

// Verify handles GET /api/v1/auth/verify.
func (h *AuthHandler) Verify(c *gin.Context) {
	token := c.Query("token")
	if err := h.svc.VerifyEmail(c.Request.Context(), token); err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "email verified successfully"})
}

// writeServiceError maps domain errors to HTTP responses.
func (h *AuthHandler) writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, utils.ErrValidation):
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: err.Error()})
	case errors.Is(err, utils.ErrEmailExists):
		c.JSON(http.StatusConflict, utils.ErrorResponse{Error: "email already registered"})
	case errors.Is(err, utils.ErrInvalidToken):
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: "invalid or expired verification token"})
	default:
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse{Error: "internal server error"})
	}
}
