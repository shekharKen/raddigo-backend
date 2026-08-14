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

// addressService abstracts user address CRUD logic.
type addressService interface {
	Create(ctx context.Context, userID string, in dto.AddressRequest) (dto.AddressResponse, error)
	List(ctx context.Context, userID string, page, pageSize int) (dto.PageResult[dto.AddressResponse], error)
	Get(ctx context.Context, userID, id string) (dto.AddressResponse, error)
	Update(ctx context.Context, userID, id string, in dto.AddressRequest) (dto.AddressResponse, error)
	Delete(ctx context.Context, userID, id string) error
}

// AddressHandler exposes user address CRUD HTTP handlers.
type AddressHandler struct {
	svc addressService
}

// NewAddressHandler creates an AddressHandler.
func NewAddressHandler(svc addressService) *AddressHandler {
	return &AddressHandler{svc: svc}
}

// Create handles POST /api/v1/users/:userId/addresses.
func (h *AddressHandler) Create(c *gin.Context) {
	var in dto.AddressRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: "invalid request body"})
		return
	}
	address, err := h.svc.Create(c.Request.Context(), c.Param("userId"), in)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"address": address})
}

// List handles GET /api/v1/users/:userId/addresses.
func (h *AddressHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	result, err := h.svc.List(c.Request.Context(), c.Param("userId"), page, pageSize)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// Get handles GET /api/v1/users/:userId/addresses/:addressId.
func (h *AddressHandler) Get(c *gin.Context) {
	address, err := h.svc.Get(c.Request.Context(), c.Param("userId"), c.Param("addressId"))
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"address": address})
}

// Update handles PUT /api/v1/users/:userId/addresses/:addressId.
func (h *AddressHandler) Update(c *gin.Context) {
	var in dto.AddressRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: "invalid request body"})
		return
	}
	address, err := h.svc.Update(c.Request.Context(), c.Param("userId"), c.Param("addressId"), in)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"address": address})
}

// Delete handles DELETE /api/v1/users/:userId/addresses/:addressId.
func (h *AddressHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("userId"), c.Param("addressId")); err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "address deleted"})
}

// writeServiceError maps domain errors to HTTP responses.
func (h *AddressHandler) writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, utils.ErrValidation):
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: err.Error()})
	case errors.Is(err, utils.ErrNotFound):
		c.JSON(http.StatusNotFound, utils.ErrorResponse{Error: "resource not found"})
	default:
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse{Error: "internal server error"})
	}
}
