package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/raddigo/raddigo/internal/dto"
	"github.com/raddigo/raddigo/internal/utils"
)

// ratingService abstracts bidirectional user/partner rating logic.
type ratingService interface {
	RatePartner(ctx context.Context, userID, partnerID string, in dto.CreateRatingRequest) (dto.RatingResponse, error)
	RateUser(ctx context.Context, partnerID, userID string, in dto.CreateRatingRequest) (dto.RatingResponse, error)
	ListForPartner(ctx context.Context, partnerID string, page, pageSize int) (dto.PageResult[dto.RatingResponse], error)
	ListForUser(ctx context.Context, userID string, page, pageSize int) (dto.PageResult[dto.RatingResponse], error)
	SummaryForPartner(ctx context.Context, partnerID string) (dto.RatingSummary, error)
	SummaryForUser(ctx context.Context, userID string) (dto.RatingSummary, error)
}

// RatingHandler exposes rating and feedback HTTP handlers.
type RatingHandler struct {
	svc ratingService
}

// NewRatingHandler creates a RatingHandler.
func NewRatingHandler(svc ratingService) *RatingHandler {
	return &RatingHandler{svc: svc}
}

// RatePartner handles POST /api/v1/user/:userId/partner/:partnerId/rating.
func (h *RatingHandler) RatePartner(c *gin.Context) {
	var in dto.CreateRatingRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: "invalid request body"})
		return
	}
	rating, err := h.svc.RatePartner(c.Request.Context(), c.Param("userId"), c.Param("partnerId"), in)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"rating": rating})
}

// RateUser handles POST /api/v1/partner/:partnerId/user/:userId/rating.
func (h *RatingHandler) RateUser(c *gin.Context) {
	var in dto.CreateRatingRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: "invalid request body"})
		return
	}
	rating, err := h.svc.RateUser(c.Request.Context(), c.Param("partnerId"), c.Param("userId"), in)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"rating": rating})
}

// ListForPartner handles GET /api/v1/partner/:partnerId/ratings.
func (h *RatingHandler) ListForPartner(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	result, err := h.svc.ListForPartner(c.Request.Context(), c.Param("partnerId"), page, pageSize)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// ListForUser handles GET /api/v1/user/:userId/ratings.
func (h *RatingHandler) ListForUser(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	result, err := h.svc.ListForUser(c.Request.Context(), c.Param("userId"), page, pageSize)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// SummaryForPartner handles GET /api/v1/partner/:partnerId/rating-summary.
func (h *RatingHandler) SummaryForPartner(c *gin.Context) {
	summary, err := h.svc.SummaryForPartner(c.Request.Context(), c.Param("partnerId"))
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"summary": summary})
}

// SummaryForUser handles GET /api/v1/user/:userId/rating-summary.
func (h *RatingHandler) SummaryForUser(c *gin.Context) {
	summary, err := h.svc.SummaryForUser(c.Request.Context(), c.Param("userId"))
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"summary": summary})
}

// writeServiceError maps domain errors to HTTP responses.
func (h *RatingHandler) writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, utils.ErrValidation):
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: err.Error()})
	case errors.Is(err, utils.ErrNotFound):
		c.JSON(http.StatusNotFound, utils.ErrorResponse{Error: "resource not found"})
	default:
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse{Error: "internal server error"})
	}
}
