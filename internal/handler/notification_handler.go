package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/raddigo/raddigo/internal/dto"
	"github.com/raddigo/raddigo/internal/middleware"
	"github.com/raddigo/raddigo/internal/model"
	"github.com/raddigo/raddigo/internal/utils"
)

// notificationService abstracts the notification inbox logic.
type notificationService interface {
	List(ctx context.Context, recipientID string, role model.NotificationRecipientRole, page, pageSize int) (dto.PageResult[dto.NotificationResponse], error)
	MarkRead(ctx context.Context, recipientID, id string) error
}

// NotificationHandler exposes the notification inbox HTTP handlers.
type NotificationHandler struct {
	svc notificationService
}

// NewNotificationHandler creates a NotificationHandler.
func NewNotificationHandler(svc notificationService) *NotificationHandler {
	return &NotificationHandler{svc: svc}
}

// ListForUser handles GET /api/v1/user/notifications.
func (h *NotificationHandler) ListForUser(c *gin.Context) {
	h.list(c, model.NotificationRecipientUser)
}

// ListForPartner handles GET /api/v1/partner/notifications.
func (h *NotificationHandler) ListForPartner(c *gin.Context) {
	h.list(c, model.NotificationRecipientPartner)
}

func (h *NotificationHandler) list(c *gin.Context, role model.NotificationRecipientRole) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	result, err := h.svc.List(c.Request.Context(), c.GetString(middleware.ContextSubjectKey), role, page, pageSize)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// MarkRead handles POST /api/v1/user/notifications/:notificationId/read and
// POST /api/v1/partner/notifications/:notificationId/read.
func (h *NotificationHandler) MarkRead(c *gin.Context) {
	recipientID := c.GetString(middleware.ContextSubjectKey)
	if err := h.svc.MarkRead(c.Request.Context(), recipientID, c.Param("notificationId")); err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "notification marked as read"})
}

func (h *NotificationHandler) writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, utils.ErrNotFound):
		c.JSON(http.StatusNotFound, utils.ErrorResponse{Error: err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse{Error: "internal server error"})
	}
}
