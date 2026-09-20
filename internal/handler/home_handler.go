package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/raddigo/raddigo/internal/dto"
	"github.com/raddigo/raddigo/internal/middleware"
	"github.com/raddigo/raddigo/internal/utils"
)

// homeService abstracts the homepage aggregation logic for both roles.
type homeService interface {
	ForUser(ctx context.Context, userID string) (dto.UserHomeResponse, error)
	ForPartner(ctx context.Context, partnerID string) (dto.PartnerHomeResponse, error)
}

// HomeHandler exposes the customer and partner homepage HTTP handlers.
type HomeHandler struct {
	svc homeService
}

// NewHomeHandler creates a HomeHandler.
func NewHomeHandler(svc homeService) *HomeHandler {
	return &HomeHandler{svc: svc}
}

// ForUser handles GET /api/v1/user/home.
func (h *HomeHandler) ForUser(c *gin.Context) {
	res, err := h.svc.ForUser(c.Request.Context(), c.GetString(middleware.ContextSubjectKey))
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// ForPartner handles GET /api/v1/partner/home.
func (h *HomeHandler) ForPartner(c *gin.Context) {
	res, err := h.svc.ForPartner(c.Request.Context(), c.GetString(middleware.ContextSubjectKey))
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *HomeHandler) writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, utils.ErrNotFound):
		c.JSON(http.StatusNotFound, utils.ErrorResponse{Error: err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse{Error: "internal server error"})
	}
}
