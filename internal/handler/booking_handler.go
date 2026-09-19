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

// bookingService abstracts slot-booking logic between users and partners.
type bookingService interface {
	Create(ctx context.Context, userID string, in dto.CreateBookingRequest) (dto.BookingResponse, error)
	ListForUser(ctx context.Context, userID string, page, pageSize int) (dto.PageResult[dto.BookingResponse], error)
	ListForPartner(ctx context.Context, partnerID, status string, page, pageSize int) (dto.PageResult[dto.BookingResponse], error)
	Accept(ctx context.Context, partnerID, bookingID string) (dto.BookingResponse, error)
	Reject(ctx context.Context, partnerID, bookingID string) (dto.BookingResponse, error)
	Update(ctx context.Context, userID, bookingID string, in dto.UpdateBookingRequest) (dto.BookingResponse, error)
	Cancel(ctx context.Context, userID, bookingID string) (dto.BookingResponse, error)
}

// BookingHandler exposes slot-booking HTTP handlers.
type BookingHandler struct {
	svc       bookingService
	uploadDir string
	baseURL   string
}

// NewBookingHandler creates a BookingHandler. uploadDir is where scrap images
// are written; publicBaseURL is the externally reachable base URL used to build
// the stored image URL.
func NewBookingHandler(svc bookingService, uploadDir, publicBaseURL string) *BookingHandler {
	return &BookingHandler{svc: svc, uploadDir: uploadDir, baseURL: publicBaseURL}
}

// Create handles POST /api/v1/user/bookings. It accepts a multipart/form-data
// body carrying the booking fields plus one or more scrap images in the
// repeated "images" field. The booking is attributed to the authenticated user.
func (h *BookingHandler) Create(c *gin.Context) {
	in := dto.CreateBookingRequest{
		PartnerID:     c.PostForm("partner_id"),
		SlotDate:      c.PostForm("slot_date"),
		SlotStartTime: c.PostForm("slot_start_time"),
		SlotEndTime:   c.PostForm("slot_end_time"),
		PickupAddress: c.PostForm("pickup_address"),
		Description:   c.PostForm("description"),
		Note:          c.PostForm("note"),
	}

	lat, err := strconv.ParseFloat(c.PostForm("pickup_latitude"), 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: "pickup_latitude is required and must be a number"})
		return
	}
	lng, err := strconv.ParseFloat(c.PostForm("pickup_longitude"), 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: "pickup_longitude is required and must be a number"})
		return
	}
	in.PickupLatitude = lat
	in.PickupLongitude = lng

	imageURLs, ok := saveUploadedImages(c, "images", h.uploadDir, h.baseURL, dto.MaxBookingImages)
	if !ok {
		return
	}
	in.ScrapImages = imageURLs

	booking, err := h.svc.Create(c.Request.Context(), c.GetString(middleware.ContextSubjectKey), in)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"booking": booking})
}

// ListForUser handles GET /api/v1/user/bookings.
func (h *BookingHandler) ListForUser(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	result, err := h.svc.ListForUser(c.Request.Context(), c.GetString(middleware.ContextSubjectKey), page, pageSize)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// ListForPartner handles GET /api/v1/partner/bookings, optionally filtered by
// ?status=pending|accepted|rejected.
func (h *BookingHandler) ListForPartner(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	result, err := h.svc.ListForPartner(c.Request.Context(), c.GetString(middleware.ContextSubjectKey), c.Query("status"), page, pageSize)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// Accept handles POST /api/v1/partner/bookings/:bookingId/accept.
func (h *BookingHandler) Accept(c *gin.Context) {
	booking, err := h.svc.Accept(c.Request.Context(), c.GetString(middleware.ContextSubjectKey), c.Param("bookingId"))
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"booking": booking})
}

// Reject handles POST /api/v1/partner/bookings/:bookingId/reject.
func (h *BookingHandler) Reject(c *gin.Context) {
	booking, err := h.svc.Reject(c.Request.Context(), c.GetString(middleware.ContextSubjectKey), c.Param("bookingId"))
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"booking": booking})
}

// Update handles PUT /api/v1/user/bookings/:bookingId. Like Create, it accepts
// a multipart/form-data body; the "images" field is optional here and, when
// present, wholesale replaces the booking's scrap images.
func (h *BookingHandler) Update(c *gin.Context) {
	in := dto.UpdateBookingRequest{
		SlotDate:      c.PostForm("slot_date"),
		SlotStartTime: c.PostForm("slot_start_time"),
		SlotEndTime:   c.PostForm("slot_end_time"),
		PickupAddress: c.PostForm("pickup_address"),
		Description:   c.PostForm("description"),
		Note:          c.PostForm("note"),
	}

	lat, err := strconv.ParseFloat(c.PostForm("pickup_latitude"), 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: "pickup_latitude is required and must be a number"})
		return
	}
	lng, err := strconv.ParseFloat(c.PostForm("pickup_longitude"), 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: "pickup_longitude is required and must be a number"})
		return
	}
	in.PickupLatitude = lat
	in.PickupLongitude = lng

	if form, ferr := c.MultipartForm(); ferr == nil && len(form.File["images"]) > 0 {
		imageURLs, ok := saveUploadedImages(c, "images", h.uploadDir, h.baseURL, dto.MaxBookingImages)
		if !ok {
			return
		}
		in.ScrapImages = imageURLs
	}

	booking, err := h.svc.Update(c.Request.Context(), c.GetString(middleware.ContextSubjectKey), c.Param("bookingId"), in)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"booking": booking})
}

// Cancel handles POST /api/v1/user/bookings/:bookingId/cancel.
func (h *BookingHandler) Cancel(c *gin.Context) {
	booking, err := h.svc.Cancel(c.Request.Context(), c.GetString(middleware.ContextSubjectKey), c.Param("bookingId"))
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"booking": booking})
}

// writeServiceError maps domain errors to HTTP responses.
func (h *BookingHandler) writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, utils.ErrValidation):
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: err.Error()})
	case errors.Is(err, utils.ErrNotFound):
		c.JSON(http.StatusNotFound, utils.ErrorResponse{Error: "resource not found"})
	case errors.Is(err, utils.ErrSlotUnavailable):
		c.JSON(http.StatusConflict, utils.ErrorResponse{Error: "slot is no longer available"})
	case errors.Is(err, utils.ErrInvalidState):
		c.JSON(http.StatusConflict, utils.ErrorResponse{Error: "booking is not in a state that allows this action"})
	default:
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse{Error: "internal server error"})
	}
}
