package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/raddigo/raddigo/internal/dto"
	"github.com/raddigo/raddigo/internal/model"
	"github.com/raddigo/raddigo/internal/utils"
)

// partnerService abstracts partner registration and verification logic.
type partnerService interface {
	Register(ctx context.Context, in dto.RegisterPartnerRequest) (model.Partner, error)
	VerifyEmail(ctx context.Context, token string) error
	SearchByLocation(ctx context.Context, lat, lng float64, page, pageSize int) (dto.PageResult[dto.PartnerSearchResult], error)
}

// PartnerHandler exposes partner authentication-related HTTP handlers.
type PartnerHandler struct {
	svc partnerService
}

// NewPartnerHandler creates a PartnerHandler.
func NewPartnerHandler(svc partnerService) *PartnerHandler {
	return &PartnerHandler{svc: svc}
}

// Register handles POST /api/v1/auth/partner/register.
func (h *PartnerHandler) Register(c *gin.Context) {
	var in dto.RegisterPartnerRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: "invalid request body"})
		return
	}

	partner, err := h.svc.Register(c.Request.Context(), in)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "registration successful, please check your email to verify your account",
		"partner": partner,
		"token":   partner.VerifyToken,
	})
}

// Verify handles GET /api/v1/auth/partner/verify.
func (h *PartnerHandler) Verify(c *gin.Context) {
	token := c.Query("token")
	if err := h.svc.VerifyEmail(c.Request.Context(), token); err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "email verified successfully"})
}

// whose operating area covers the caller's current location.
func (h *PartnerHandler) Search(c *gin.Context) {
	lat, err := strconv.ParseFloat(c.Query("latitude"), 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: "latitude is required and must be a number"})
		return
	}
	lng, err := strconv.ParseFloat(c.Query("longitude"), 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: "longitude is required and must be a number"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	result, err := h.svc.SearchByLocation(c.Request.Context(), lat, lng, page, pageSize)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// writeServiceError maps domain errors to HTTP responses.
func (h *PartnerHandler) writeServiceError(c *gin.Context, err error) {
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
