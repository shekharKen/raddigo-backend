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
	ForgotPassword(ctx context.Context, in dto.ForgotPasswordRequest) error
	ResetPassword(ctx context.Context, in dto.ResetPasswordRequest) error
}

// authService abstracts credential login and token refresh logic.
type authService interface {
	LoginUser(ctx context.Context, in dto.LoginRequest) (dto.AuthResponse, error)
	LoginPartner(ctx context.Context, in dto.LoginRequest) (dto.AuthResponse, error)
	Refresh(refreshToken string) (dto.AuthResponse, error)
	IssueForUser(userID string) (dto.AuthResponse, error)
	IssueForPartner(partnerID string) (dto.AuthResponse, error)
}

// AuthHandler exposes authentication-related HTTP handlers.
type AuthHandler struct {
	svc  userService
	auth authService
}

// NewAuthHandler creates an AuthHandler.
func NewAuthHandler(svc userService, auth authService) *AuthHandler {
	return &AuthHandler{svc: svc, auth: auth}
}

// Register handles POST /api/v1/auth/user/register.
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

	tokens, err := h.auth.IssueForUser(user.ID)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "registration successful, please check your email to verify your account",
		"user":    user,
		"auth":    tokens,
	})
}

// Login handles POST /api/v1/auth/user/login.
func (h *AuthHandler) Login(c *gin.Context) {
	var in dto.LoginRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: "invalid request body"})
		return
	}
	tokens, err := h.auth.LoginUser(c.Request.Context(), in)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"auth": tokens})
}

// Refresh handles POST /api/v1/auth/refresh for users and partners alike.
func (h *AuthHandler) Refresh(c *gin.Context) {
	var in dto.RefreshRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: "invalid request body"})
		return
	}
	tokens, err := h.auth.Refresh(in.RefreshToken)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"auth": tokens})
}

// Verify handles GET /api/v1/auth/user/verify.
func (h *AuthHandler) Verify(c *gin.Context) {
	token := c.Query("token")
	if err := h.svc.VerifyEmail(c.Request.Context(), token); err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "email verified successfully"})
}

// ForgotPassword handles POST /api/v1/auth/user/forgot-password.
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var in dto.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: "invalid request body"})
		return
	}
	if err := h.svc.ForgotPassword(c.Request.Context(), in); err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "if the email is registered, a password reset link has been sent"})
}

// ResetPassword handles POST /api/v1/auth/user/reset-password.
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var in dto.ResetPasswordRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: "invalid request body"})
		return
	}
	if err := h.svc.ResetPassword(c.Request.Context(), in); err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "password has been reset successfully"})
}

// writeServiceError maps domain errors to HTTP responses.
func (h *AuthHandler) writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, utils.ErrValidation):
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: err.Error()})
	case errors.Is(err, utils.ErrEmailExists):
		c.JSON(http.StatusConflict, utils.ErrorResponse{Error: "email already registered"})
	case errors.Is(err, utils.ErrInvalidCredentials):
		c.JSON(http.StatusUnauthorized, utils.ErrorResponse{Error: "invalid email or password"})
	case errors.Is(err, utils.ErrNotVerified):
		c.JSON(http.StatusForbidden, utils.ErrorResponse{Error: "account not verified, please verify your email"})
	case errors.Is(err, utils.ErrInvalidToken):
		c.JSON(http.StatusUnauthorized, utils.ErrorResponse{Error: "invalid or expired token"})
	default:
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse{Error: "internal server error"})
	}
}
