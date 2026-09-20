package handler

import (
	"context"
	"errors"
	"io"
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
	GetForUser(ctx context.Context, userID, bookingID string) (dto.BookingResponse, error)
	GetForPartner(ctx context.Context, partnerID, bookingID string) (dto.BookingResponse, error)
	ListForUser(ctx context.Context, userID, status string, page, pageSize int) (dto.PageResult[dto.BookingResponse], error)
	ListForPartner(ctx context.Context, partnerID, status string, page, pageSize int) (dto.PageResult[dto.BookingResponse], error)
	NextForUser(ctx context.Context, userID string) (dto.BookingResponse, error)
	NextForPartner(ctx context.Context, partnerID string) (dto.BookingResponse, error)
	Accept(ctx context.Context, partnerID, bookingID string) (dto.BookingResponse, error)
	Reject(ctx context.Context, partnerID, bookingID, reason string) (dto.BookingResponse, error)
	Update(ctx context.Context, userID, bookingID string, in dto.UpdateBookingRequest) (dto.BookingResponse, error)
	Cancel(ctx context.Context, userID, bookingID, reason string) (dto.BookingResponse, error)
	SendOTP(ctx context.Context, partnerID, bookingID string) (dto.BookingResponse, error)
	VerifyOTP(ctx context.Context, partnerID, bookingID string, in dto.VerifyBookingOTPRequest) (dto.BookingResponse, error)
	Complete(ctx context.Context, partnerID, bookingID string, in dto.CompleteBookingRequest) (dto.BookingResponse, error)
	StatsForUser(ctx context.Context, userID string) (dto.UserStatsResponse, error)
	StatsForPartner(ctx context.Context, partnerID string) (dto.PartnerStatsResponse, error)
}

// BookingHandler exposes slot-booking HTTP handlers.
type BookingHandler struct {
	svc       bookingService
	uploadDir string
}

// NewBookingHandler creates a BookingHandler. uploadDir is where scrap images
// are written.
func NewBookingHandler(svc bookingService, uploadDir string) *BookingHandler {
	return &BookingHandler{svc: svc, uploadDir: uploadDir}
}

// Create handles POST /api/v1/user/bookings. It accepts a multipart/form-data
// body carrying the booking fields plus zero or more scrap images in the
// repeated "images" field (images are optional for now). The booking is
// attributed to the authenticated user.
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

	imageURLs, ok := saveUploadedImages(c, "images", h.uploadDir, dto.MaxBookingImages)
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

// GetForUser handles GET /api/v1/user/bookings/:bookingId.
func (h *BookingHandler) GetForUser(c *gin.Context) {
	booking, err := h.svc.GetForUser(c.Request.Context(), c.GetString(middleware.ContextSubjectKey), c.Param("bookingId"))
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"booking": booking})
}

// GetForPartner handles GET /api/v1/partner/bookings/:bookingId.
func (h *BookingHandler) GetForPartner(c *gin.Context) {
	booking, err := h.svc.GetForPartner(c.Request.Context(), c.GetString(middleware.ContextSubjectKey), c.Param("bookingId"))
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"booking": booking})
}

// ListForUser handles GET /api/v1/user/bookings, optionally filtered by
// ?status=pending|accepted|rejected|cancelled|completed.
func (h *BookingHandler) ListForUser(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	result, err := h.svc.ListForUser(c.Request.Context(), c.GetString(middleware.ContextSubjectKey), c.Query("status"), page, pageSize)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// NextForUser handles GET /api/v1/user/bookings/next. It returns the user's
// next scheduled (accepted) pickup with full details: partner details and the
// booking's status log history.
func (h *BookingHandler) NextForUser(c *gin.Context) {
	booking, err := h.svc.NextForUser(c.Request.Context(), c.GetString(middleware.ContextSubjectKey))
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"booking": booking})
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

// NextForPartner handles GET /api/v1/partner/bookings/next. It returns the
// partner's next scheduled (accepted) pickup with full details: the
// customer's details and the booking's status log history.
func (h *BookingHandler) NextForPartner(c *gin.Context) {
	booking, err := h.svc.NextForPartner(c.Request.Context(), c.GetString(middleware.ContextSubjectKey))
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"booking": booking})
}

// StatsForUser handles GET /api/v1/user/stats.
func (h *BookingHandler) StatsForUser(c *gin.Context) {
	stats, err := h.svc.StatsForUser(c.Request.Context(), c.GetString(middleware.ContextSubjectKey))
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"stats": stats})
}

// StatsForPartner handles GET /api/v1/partner/stats.
func (h *BookingHandler) StatsForPartner(c *gin.Context) {
	stats, err := h.svc.StatsForPartner(c.Request.Context(), c.GetString(middleware.ContextSubjectKey))
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"stats": stats})
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

// Reject handles POST /api/v1/partner/bookings/:bookingId/reject. The
// request body is optional JSON, e.g. {"reason": "..."}.
func (h *BookingHandler) Reject(c *gin.Context) {
	var in dto.RejectBookingRequest
	if err := c.ShouldBindJSON(&in); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: "invalid request body"})
		return
	}

	booking, err := h.svc.Reject(c.Request.Context(), c.GetString(middleware.ContextSubjectKey), c.Param("bookingId"), in.Reason)
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
		imageURLs, ok := saveUploadedImages(c, "images", h.uploadDir, dto.MaxBookingImages)
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

// Cancel handles POST /api/v1/user/bookings/:bookingId/cancel. The request
// body is optional JSON, e.g. {"reason": "..."}.
func (h *BookingHandler) Cancel(c *gin.Context) {
	var in dto.CancelBookingRequest
	if err := c.ShouldBindJSON(&in); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: "invalid request body"})
		return
	}

	booking, err := h.svc.Cancel(c.Request.Context(), c.GetString(middleware.ContextSubjectKey), c.Param("bookingId"), in.Reason)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"booking": booking})
}

// SendOTP handles POST /api/v1/partner/bookings/:bookingId/send-otp. It
// generates and emails a fresh completion OTP to the customer for an
// accepted booking, for the partner to read back later via VerifyOTP.
func (h *BookingHandler) SendOTP(c *gin.Context) {
	booking, err := h.svc.SendOTP(c.Request.Context(), c.GetString(middleware.ContextSubjectKey), c.Param("bookingId"))
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"booking": booking})
}

// VerifyOTP handles POST /api/v1/partner/bookings/:bookingId/verify-otp. The
// partner submits the OTP read back from the customer to confirm the handoff;
// this must succeed before Complete will accept the booking's completion
// details.
func (h *BookingHandler) VerifyOTP(c *gin.Context) {
	var in dto.VerifyBookingOTPRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: "invalid request body"})
		return
	}

	booking, err := h.svc.VerifyOTP(c.Request.Context(), c.GetString(middleware.ContextSubjectKey), c.Param("bookingId"), in)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"booking": booking})
}

// Complete handles POST /api/v1/partner/bookings/:bookingId/complete. It
// accepts a multipart/form-data body carrying the scrap images collected at
// pickup (repeated "images" field, at least one required) plus the weighed
// scrap's weight and the amount paid to the customer. The booking's
// completion OTP must have already been verified via VerifyOTP.
func (h *BookingHandler) Complete(c *gin.Context) {
	weight, err := strconv.ParseFloat(c.PostForm("weight"), 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: "weight is required and must be a number"})
		return
	}
	weightUnit := c.PostForm("weight_unit")
	amountPaid, err := strconv.ParseFloat(c.PostForm("amount_paid"), 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: "amount_paid is required and must be a number"})
		return
	}

	imageURLs, ok := saveUploadedImages(c, "images", h.uploadDir, dto.MaxCompletionImages)
	if !ok {
		return
	}

	in := dto.CompleteBookingRequest{
		Weight:     weight,
		WeightUnit: weightUnit,
		AmountPaid: amountPaid,
		Images:     imageURLs,
	}

	booking, err := h.svc.Complete(c.Request.Context(), c.GetString(middleware.ContextSubjectKey), c.Param("bookingId"), in)
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
	case errors.Is(err, utils.ErrOTPNotVerified):
		c.JSON(http.StatusConflict, utils.ErrorResponse{Error: "completion otp must be verified before completing this booking"})
	case errors.Is(err, utils.ErrInvalidOTP):
		c.JSON(http.StatusBadRequest, utils.ErrorResponse{Error: "invalid or expired otp"})
	default:
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse{Error: "internal server error"})
	}
}
