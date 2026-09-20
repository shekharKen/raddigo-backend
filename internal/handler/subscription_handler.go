package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/raddigo/raddigo/internal/dto"
	"github.com/raddigo/raddigo/internal/middleware"
	"github.com/raddigo/raddigo/internal/utils"
)

// subscriptionService abstracts partner subscription, upgrade and billing logic.
type subscriptionService interface {
	Subscribe(ctx context.Context, partnerID string, in dto.CreateSubscriptionRequest) (dto.SubscriptionWithTransaction, error)
	Upgrade(ctx context.Context, partnerID string, in dto.UpgradeSubscriptionRequest) (dto.SubscriptionWithTransaction, error)
	GetActive(ctx context.Context, partnerID string) (dto.SubscriptionResponse, error)
	ListForPartner(ctx context.Context, partnerID string, page, pageSize int) (dto.PageResult[dto.SubscriptionResponse], error)
	ListTransactions(ctx context.Context, partnerID string, page, pageSize int) (dto.PageResult[dto.TransactionResponse], error)
}

// SubscriptionHandler exposes partner subscription HTTP handlers.
type SubscriptionHandler struct {
	svc subscriptionService
}

// NewSubscriptionHandler creates a SubscriptionHandler.
func NewSubscriptionHandler(svc subscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{svc: svc}
}

// Subscribe handles POST /api/v1/partner/subscription.
func (h *SubscriptionHandler) Subscribe(c *gin.Context) {
	var in dto.CreateSubscriptionRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: "invalid request body"})
		return
	}
	result, err := h.svc.Subscribe(c.Request.Context(), c.GetString(middleware.ContextSubjectKey), in)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, result)
}

// Upgrade handles POST /api/v1/partner/subscription/upgrade.
func (h *SubscriptionHandler) Upgrade(c *gin.Context) {
	var in dto.UpgradeSubscriptionRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: "invalid request body"})
		return
	}
	result, err := h.svc.Upgrade(c.Request.Context(), c.GetString(middleware.ContextSubjectKey), in)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetActive handles GET /api/v1/partner/subscription.
func (h *SubscriptionHandler) GetActive(c *gin.Context) {
	sub, err := h.svc.GetActive(c.Request.Context(), c.GetString(middleware.ContextSubjectKey))
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"subscription": sub})
}

// List handles GET /api/v1/partner/subscriptions.
func (h *SubscriptionHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	result, err := h.svc.ListForPartner(c.Request.Context(), c.GetString(middleware.ContextSubjectKey), page, pageSize)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// ListTransactions handles GET /api/v1/partner/subscription/transactions.
func (h *SubscriptionHandler) ListTransactions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	result, err := h.svc.ListTransactions(c.Request.Context(), c.GetString(middleware.ContextSubjectKey), page, pageSize)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// writeServiceError maps domain errors to HTTP responses.
func (h *SubscriptionHandler) writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, utils.ErrValidation):
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: err.Error()})
	case errors.Is(err, utils.ErrNotFound):
		c.JSON(http.StatusNotFound, utils.ErrorResponse{Error: "resource not found"})
	case errors.Is(err, utils.ErrInvalidState):
		c.JSON(http.StatusConflict, utils.ErrorResponse{Error: "subscription is not in a state that allows this action"})
	default:
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse{Error: "internal server error"})
	}
}
