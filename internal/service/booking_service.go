package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/raddigo/raddigo/internal/dto"
	"github.com/raddigo/raddigo/internal/model"
	"github.com/raddigo/raddigo/internal/repository"
	"github.com/raddigo/raddigo/internal/utils"
	"github.com/raddigo/raddigo/internal/validation"
)

// BookingService contains slot-booking logic: users request a partner's slot
// and partners accept or reject the request.
type BookingService struct {
	repo         repository.BookingRepository
	partners     repository.PartnerRepository
	slotDuration time.Duration
	now          func() time.Time
	id           func() string
}

// NewBookingService creates a BookingService. slotDuration must match the value
// used to build a partner's available slots so requested slots can be validated.
func NewBookingService(repo repository.BookingRepository, partners repository.PartnerRepository, slotDuration time.Duration) *BookingService {
	if slotDuration <= 0 {
		slotDuration = 30 * time.Minute
	}
	return &BookingService{
		repo:         repo,
		partners:     partners,
		slotDuration: slotDuration,
		now:          time.Now,
		id:           func() string { return uuid.NewString() },
	}
}

// Create validates the request, confirms the requested slot is a real,
// currently-available slot of the partner, and stores a pending booking.
func (s *BookingService) Create(ctx context.Context, userID string, in dto.CreateBookingRequest) (dto.BookingResponse, error) {
	if err := validation.ValidateCreateBooking(in); err != nil {
		return dto.BookingResponse{}, err
	}

	partnerID := strings.TrimSpace(in.PartnerID)
	partner, err := s.partners.GetByID(ctx, partnerID)
	if err != nil {
		return dto.BookingResponse{}, err
	}

	start := strings.TrimSpace(in.SlotStartTime)
	end := strings.TrimSpace(in.SlotEndTime)
	if !slotIsValid(partner.StartTime, partner.EndTime, start, end, s.slotDuration) {
		return dto.BookingResponse{}, utils.NewValidationError("requested slot is not a valid slot within the partner's working hours")
	}

	slotDate := strings.TrimSpace(in.SlotDate)
	taken, err := s.repo.SlotAccepted(ctx, partnerID, slotDate, start)
	if err != nil {
		return dto.BookingResponse{}, err
	}
	if taken {
		return dto.BookingResponse{}, utils.ErrSlotUnavailable
	}

	now := s.now()
	booking := model.Booking{
		ID:              s.id(),
		UserID:          userID,
		PartnerID:       partnerID,
		SlotDate:        slotDate,
		SlotStartTime:   start,
		SlotEndTime:     end,
		Status:          model.BookingPending,
		PickupLatitude:  in.PickupLatitude,
		PickupLongitude: in.PickupLongitude,
		PickupAddress:   strings.TrimSpace(in.PickupAddress),
		ScrapImage:      in.ScrapImage,
		Description:     strings.TrimSpace(in.Description),
		Note:            strings.TrimSpace(in.Note),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.repo.Create(ctx, &booking); err != nil {
		return dto.BookingResponse{}, err
	}

	booking.Partner = &partner
	return toBookingResponse(booking), nil
}

// ListForUser returns a user's bookings, each with its partner details.
func (s *BookingService) ListForUser(ctx context.Context, userID string, page, pageSize int) (dto.PageResult[dto.BookingResponse], error) {
	page, pageSize = dto.NormalizePageParams(page, pageSize)
	offset := (page - 1) * pageSize

	bookings, total, err := s.repo.ListByUser(ctx, userID, pageSize, offset)
	if err != nil {
		return dto.PageResult[dto.BookingResponse]{}, err
	}
	return toBookingPage(bookings, page, pageSize, total), nil
}

// ListForPartner returns a partner's booking requests, optionally filtered by
// status, each with the requesting user's limited details.
func (s *BookingService) ListForPartner(ctx context.Context, partnerID, status string, page, pageSize int) (dto.PageResult[dto.BookingResponse], error) {
	filter, err := parseBookingStatus(status)
	if err != nil {
		return dto.PageResult[dto.BookingResponse]{}, err
	}

	page, pageSize = dto.NormalizePageParams(page, pageSize)
	offset := (page - 1) * pageSize

	bookings, total, err := s.repo.ListByPartner(ctx, partnerID, filter, pageSize, offset)
	if err != nil {
		return dto.PageResult[dto.BookingResponse]{}, err
	}
	return toBookingPage(bookings, page, pageSize, total), nil
}

// Accept marks a partner's pending booking as accepted, disabling that slot.
func (s *BookingService) Accept(ctx context.Context, partnerID, bookingID string) (dto.BookingResponse, error) {
	booking, err := s.repo.Accept(ctx, bookingID, partnerID)
	if err != nil {
		return dto.BookingResponse{}, err
	}
	return toBookingResponse(booking), nil
}

// Reject marks a partner's pending booking as rejected.
func (s *BookingService) Reject(ctx context.Context, partnerID, bookingID string) (dto.BookingResponse, error) {
	booking, err := s.repo.Reject(ctx, bookingID, partnerID)
	if err != nil {
		return dto.BookingResponse{}, err
	}
	return toBookingResponse(booking), nil
}

// Update validates the request and updates a user's pending booking's slot
// and pickup details, re-checking the new slot against the partner's working
// hours and against any already-accepted competing booking.
func (s *BookingService) Update(ctx context.Context, userID, bookingID string, in dto.UpdateBookingRequest) (dto.BookingResponse, error) {
	if err := validation.ValidateUpdateBooking(in); err != nil {
		return dto.BookingResponse{}, err
	}

	booking, err := s.repo.GetByID(ctx, bookingID)
	if err != nil {
		return dto.BookingResponse{}, err
	}
	if booking.UserID != userID {
		return dto.BookingResponse{}, utils.ErrNotFound
	}

	partner, err := s.partners.GetByID(ctx, booking.PartnerID)
	if err != nil {
		return dto.BookingResponse{}, err
	}

	start := strings.TrimSpace(in.SlotStartTime)
	end := strings.TrimSpace(in.SlotEndTime)
	if !slotIsValid(partner.StartTime, partner.EndTime, start, end, s.slotDuration) {
		return dto.BookingResponse{}, utils.NewValidationError("requested slot is not a valid slot within the partner's working hours")
	}

	updated, err := s.repo.Update(ctx, bookingID, userID, repository.BookingUpdateInput{
		SlotDate:        strings.TrimSpace(in.SlotDate),
		SlotStartTime:   start,
		SlotEndTime:     end,
		PickupLatitude:  in.PickupLatitude,
		PickupLongitude: in.PickupLongitude,
		PickupAddress:   strings.TrimSpace(in.PickupAddress),
		ScrapImage:      in.ScrapImage,
		Description:     strings.TrimSpace(in.Description),
		Note:            strings.TrimSpace(in.Note),
	})
	if err != nil {
		return dto.BookingResponse{}, err
	}
	return toBookingResponse(updated), nil
}

// Cancel marks a user's pending or accepted booking as cancelled.
func (s *BookingService) Cancel(ctx context.Context, userID, bookingID string) (dto.BookingResponse, error) {
	booking, err := s.repo.Cancel(ctx, bookingID, userID)
	if err != nil {
		return dto.BookingResponse{}, err
	}
	return toBookingResponse(booking), nil
}

// slotIsValid reports whether start/end exactly matches one of the slots
// generated from the partner's working window and slot duration.
func slotIsValid(workStart, workEnd, start, end string, dur time.Duration) bool {
	for _, slot := range buildTimeSlots(workStart, workEnd, dur) {
		if slot.StartTime == start && slot.EndTime == end {
			return true
		}
	}
	return false
}

// parseBookingStatus maps an optional status filter string to a BookingStatus.
func parseBookingStatus(status string) (model.BookingStatus, error) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "":
		return "", nil
	case string(model.BookingPending):
		return model.BookingPending, nil
	case string(model.BookingAccepted):
		return model.BookingAccepted, nil
	case string(model.BookingRejected):
		return model.BookingRejected, nil
	case string(model.BookingCancelled):
		return model.BookingCancelled, nil
	default:
		return "", utils.NewValidationError("status is invalid: must be one of pending, accepted, rejected or cancelled")
	}
}

func toBookingPage(bookings []model.Booking, page, pageSize int, total int64) dto.PageResult[dto.BookingResponse] {
	out := make([]dto.BookingResponse, 0, len(bookings))
	for _, b := range bookings {
		out = append(out, toBookingResponse(b))
	}
	return dto.PageResult[dto.BookingResponse]{
		Data:       out,
		Pagination: dto.NewPagination(page, pageSize, total),
	}
}

func toBookingResponse(b model.Booking) dto.BookingResponse {
	res := dto.BookingResponse{
		ID:              b.ID,
		Status:          string(b.Status),
		SlotDate:        b.SlotDate,
		SlotStartTime:   b.SlotStartTime,
		SlotEndTime:     b.SlotEndTime,
		PickupLatitude:  b.PickupLatitude,
		PickupLongitude: b.PickupLongitude,
		PickupAddress:   b.PickupAddress,
		ScrapImage:      b.ScrapImage,
		Description:     b.Description,
		Note:            b.Note,
		CreatedAt:       b.CreatedAt,
		UpdatedAt:       b.UpdatedAt,
	}
	if b.User != nil {
		res.User = &dto.BookingUser{
			ID:              b.User.ID,
			FirstName:       b.User.FirstName,
			LastName:        b.User.LastName,
			MobileExtension: b.User.MobileExtension,
			MobileNo:        b.User.MobileNo,
		}
	}
	if b.Partner != nil {
		res.Partner = &dto.BookingPartner{
			ID:              b.Partner.ID,
			FirstName:       b.Partner.FirstName,
			LastName:        b.Partner.LastName,
			StoreName:       b.Partner.StoreName,
			MobileExtension: b.Partner.MobileExtension,
			MobileNo:        b.Partner.MobileNo,
		}
	}
	return res
}
