package service

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/raddigo/raddigo/internal/dto"
	"github.com/raddigo/raddigo/internal/mailer"
	"github.com/raddigo/raddigo/internal/model"
	"github.com/raddigo/raddigo/internal/repository"
	"github.com/raddigo/raddigo/internal/utils"
	"github.com/raddigo/raddigo/internal/validation"
)

// bookingOTPTTL is how long a booking completion OTP remains valid once the
// partner accepts the booking.
const bookingOTPTTL = 2 * time.Hour

// BookingService contains slot-booking logic: users request a partner's slot
// and partners accept or reject the request.
type BookingService struct {
	repo          repository.BookingRepository
	partners      repository.PartnerRepository
	ratings       repository.RatingRepository
	mailer        mailer.Mailer
	notifications *NotificationService
	slotDuration  time.Duration
	baseURL       string
	now           func() time.Time
	id            func() string
	otp           func() (string, error)
}

// NewBookingService creates a BookingService. slotDuration must match the value
// used to build a partner's available slots so requested slots can be validated.
// baseURL is prefixed onto stored relative image paths when returning them to
// clients. devOTP, when non-empty, is used as a fixed completion OTP instead of
// a random one (development only).
func NewBookingService(repo repository.BookingRepository, partners repository.PartnerRepository, ratings repository.RatingRepository, slotDuration time.Duration, baseURL string, m mailer.Mailer, devOTP string, notifications *NotificationService) *BookingService {
	if slotDuration <= 0 {
		slotDuration = 30 * time.Minute
	}
	return &BookingService{
		repo:          repo,
		partners:      partners,
		ratings:       ratings,
		mailer:        m,
		notifications: notifications,
		slotDuration:  slotDuration,
		baseURL:       baseURL,
		now:           time.Now,
		id:            func() string { return uuid.NewString() },
		otp:           newOTPFunc(devOTP),
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
		Description:     strings.TrimSpace(in.Description),
		Note:            strings.TrimSpace(in.Note),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	booking.Images = buildBookingImages(booking.ID, in.ScrapImages, now, s.id)
	if err := s.repo.Create(ctx, &booking); err != nil {
		return dto.BookingResponse{}, err
	}

	booking.Partner = &partner
	s.notifications.BookingRequested(ctx, booking)
	return s.toBookingResponse(booking), nil
}

// GetForUser returns a single booking owned by the user, with its partner
// details (including their aggregate rating) and status history, or
// utils.ErrNotFound when it doesn't exist or belongs to someone else.
func (s *BookingService) GetForUser(ctx context.Context, userID, bookingID string) (dto.BookingResponse, error) {
	booking, err := s.repo.GetByID(ctx, bookingID)
	if err != nil {
		return dto.BookingResponse{}, err
	}
	if booking.UserID != userID {
		return dto.BookingResponse{}, utils.ErrNotFound
	}
	return s.bookingDetails(ctx, booking)
}

// GetForPartner returns a single booking owned by the partner, with the
// requesting user's details and status history, or utils.ErrNotFound when it
// doesn't exist or belongs to a different partner.
func (s *BookingService) GetForPartner(ctx context.Context, partnerID, bookingID string) (dto.BookingResponse, error) {
	booking, err := s.repo.GetByID(ctx, bookingID)
	if err != nil {
		return dto.BookingResponse{}, err
	}
	if booking.PartnerID != partnerID {
		return dto.BookingResponse{}, utils.ErrNotFound
	}
	return s.bookingDetails(ctx, booking)
}

// bookingDetails builds the full single-booking response: the base fields
// plus the embedded partner's rating summary and the status history. Not used
// by list endpoints to avoid an extra query per row.
func (s *BookingService) bookingDetails(ctx context.Context, booking model.Booking) (dto.BookingResponse, error) {
	res := s.toBookingResponse(booking)

	if res.Partner != nil {
		avg, total, err := s.ratings.Summary(ctx, repository.RatingFilter{
			Direction: model.RatingUserToPartner,
			PartnerID: res.Partner.ID,
		})
		if err != nil {
			return dto.BookingResponse{}, err
		}
		res.Partner.AverageRating = math.Round(avg*100) / 100
		res.Partner.TotalRatings = total
	}

	logs, err := s.listStatusLogs(ctx, booking.ID)
	if err != nil {
		return dto.BookingResponse{}, err
	}
	res.StatusLogs = logs

	ratings, err := s.bookingRatings(ctx, booking.UserID, booking.PartnerID)
	if err != nil {
		return dto.BookingResponse{}, err
	}
	res.Ratings = ratings

	return res, nil
}

// bookingRatings looks up the rating each side has given the other for this
// user/partner pair, returning nil when neither has rated yet.
func (s *BookingService) bookingRatings(ctx context.Context, userID, partnerID string) (*dto.BookingRatings, error) {
	var ratings dto.BookingRatings

	userToPartner, err := s.ratings.GetOne(ctx, repository.RatingFilter{
		Direction: model.RatingUserToPartner, UserID: userID, PartnerID: partnerID,
	})
	if err == nil {
		res := toRatingResponse(userToPartner)
		ratings.UserToPartner = &res
	} else if !errors.Is(err, utils.ErrNotFound) {
		return nil, err
	}

	partnerToUser, err := s.ratings.GetOne(ctx, repository.RatingFilter{
		Direction: model.RatingPartnerToUser, UserID: userID, PartnerID: partnerID,
	})
	if err == nil {
		res := toRatingResponse(partnerToUser)
		ratings.PartnerToUser = &res
	} else if !errors.Is(err, utils.ErrNotFound) {
		return nil, err
	}

	if ratings.UserToPartner == nil && ratings.PartnerToUser == nil {
		return nil, nil
	}
	return &ratings, nil
}

// ListForUser returns a user's bookings, optionally filtered by status, each
// with its partner details.
func (s *BookingService) ListForUser(ctx context.Context, userID, status string, page, pageSize int) (dto.PageResult[dto.BookingResponse], error) {
	filter, err := parseBookingStatus(status)
	if err != nil {
		return dto.PageResult[dto.BookingResponse]{}, err
	}

	page, pageSize = dto.NormalizePageParams(page, pageSize)
	offset := (page - 1) * pageSize

	bookings, total, err := s.repo.ListByUser(ctx, userID, filter, pageSize, offset)
	if err != nil {
		return dto.PageResult[dto.BookingResponse]{}, err
	}
	return s.toBookingPage(bookings, page, pageSize, total), nil
}

// NextForUser returns a user's next scheduled (accepted) pickup, with full
// details: partner details, and the booking's status log history.
func (s *BookingService) NextForUser(ctx context.Context, userID string) (dto.BookingResponse, error) {
	booking, err := s.repo.NextForUser(ctx, userID)
	if err != nil {
		return dto.BookingResponse{}, err
	}
	return s.bookingDetails(ctx, booking)
}

// NextForPartner returns a partner's next scheduled (accepted) pickup, with
// full details: the requesting customer's details, and the booking's status
// log history.
func (s *BookingService) NextForPartner(ctx context.Context, partnerID string) (dto.BookingResponse, error) {
	booking, err := s.repo.NextForPartner(ctx, partnerID)
	if err != nil {
		return dto.BookingResponse{}, err
	}
	return s.bookingDetails(ctx, booking)
}

// StatsForUser returns a customer's total booking count and total completion
// weight (in kg) sent across their completed bookings.
func (s *BookingService) StatsForUser(ctx context.Context, userID string) (dto.UserStatsResponse, error) {
	stats, err := s.repo.StatsByUser(ctx, userID)
	if err != nil {
		return dto.UserStatsResponse{}, err
	}
	return dto.UserStatsResponse{
		TotalBookings: stats.TotalBookings,
		TotalKgSent:   math.Round(stats.TotalWeightKg*100) / 100,
	}, nil
}

// StatsForPartner returns a partner's total completed pickups, total scrap
// collected (in kg), average rating received from customers, and total
// amount paid out to customers.
func (s *BookingService) StatsForPartner(ctx context.Context, partnerID string) (dto.PartnerStatsResponse, error) {
	stats, err := s.repo.StatsByPartner(ctx, partnerID)
	if err != nil {
		return dto.PartnerStatsResponse{}, err
	}
	avg, _, err := s.ratings.Summary(ctx, repository.RatingFilter{
		Direction: model.RatingUserToPartner,
		PartnerID: partnerID,
	})
	if err != nil {
		return dto.PartnerStatsResponse{}, err
	}
	return dto.PartnerStatsResponse{
		TotalPickups:          stats.TotalPickups,
		TotalScrapCollectedKg: math.Round(stats.TotalWeightKg*100) / 100,
		AverageRating:         math.Round(avg*100) / 100,
		AmountPaid:            math.Round(stats.AmountPaid*100) / 100,
	}, nil
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
	return s.toBookingPage(bookings, page, pageSize, total), nil
}

// Accept marks a partner's pending booking as accepted, disabling that slot.
func (s *BookingService) Accept(ctx context.Context, partnerID, bookingID string) (dto.BookingResponse, error) {
	booking, err := s.repo.Accept(ctx, bookingID, partnerID)
	if err != nil {
		return dto.BookingResponse{}, err
	}
	s.notifications.BookingAccepted(ctx, booking)
	return s.toBookingResponse(booking), nil
}

// Reject marks a partner's pending booking as rejected, optionally recording
// the partner's reason.
func (s *BookingService) Reject(ctx context.Context, partnerID, bookingID, reason string) (dto.BookingResponse, error) {
	booking, err := s.repo.Reject(ctx, bookingID, partnerID, strings.TrimSpace(reason))
	if err != nil {
		return dto.BookingResponse{}, err
	}
	s.notifications.BookingRejected(ctx, booking)
	return s.toBookingResponse(booking), nil
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
		Images:          buildBookingImagesIfProvided(bookingID, in.ScrapImages, s.now(), s.id),
		Description:     strings.TrimSpace(in.Description),
		Note:            strings.TrimSpace(in.Note),
	})
	if err != nil {
		return dto.BookingResponse{}, err
	}
	return s.toBookingResponse(updated), nil
}

// Cancel marks a user's pending or accepted booking as cancelled, optionally
// recording the user's reason.
func (s *BookingService) Cancel(ctx context.Context, userID, bookingID, reason string) (dto.BookingResponse, error) {
	booking, err := s.repo.Cancel(ctx, bookingID, userID, strings.TrimSpace(reason))
	if err != nil {
		return dto.BookingResponse{}, err
	}
	s.notifications.BookingCancelled(ctx, booking)
	return s.toBookingResponse(booking), nil
}

// CancelByPartner marks a partner's accepted booking as cancelled, optionally
// recording the partner's reason, and notifies the customer.
func (s *BookingService) CancelByPartner(ctx context.Context, partnerID, bookingID, reason string) (dto.BookingResponse, error) {
	booking, err := s.repo.CancelByPartner(ctx, bookingID, partnerID, strings.TrimSpace(reason))
	if err != nil {
		return dto.BookingResponse{}, err
	}
	s.notifications.BookingCancelledByPartner(ctx, booking)
	return s.toBookingResponse(booking), nil
}

// SendOTP generates and emails a fresh completion OTP to the customer for an
// accepted booking, for the partner to read back later via VerifyOTP.
func (s *BookingService) SendOTP(ctx context.Context, partnerID, bookingID string) (dto.BookingResponse, error) {
	otp, err := s.otp()
	if err != nil {
		return dto.BookingResponse{}, err
	}

	booking, err := s.repo.SendOTP(ctx, bookingID, partnerID, otp, s.now().Add(bookingOTPTTL))
	if err != nil {
		return dto.BookingResponse{}, err
	}

	if booking.User != nil && booking.User.Email != "" {
		if err := s.mailer.SendBookingOTP(ctx, booking.User.Email, otp); err != nil {
			return dto.BookingResponse{}, err
		}
	}
	return s.toBookingResponse(booking), nil
}

// VerifyOTP confirms the completion OTP a partner reads back from the
// customer for an accepted booking. It must succeed before Complete will
// accept the booking's completion details.
func (s *BookingService) VerifyOTP(ctx context.Context, partnerID, bookingID string, in dto.VerifyBookingOTPRequest) (dto.BookingResponse, error) {
	if err := validation.ValidateVerifyBookingOTP(in); err != nil {
		return dto.BookingResponse{}, err
	}
	booking, err := s.repo.VerifyOTP(ctx, bookingID, partnerID, strings.TrimSpace(in.OTP), s.now())
	if err != nil {
		return dto.BookingResponse{}, err
	}
	return s.toBookingResponse(booking), nil
}

// Complete validates the submitted completion details (scrap images, weight
// and amount paid) and marks a partner's accepted booking as completed. The
// booking's completion OTP must have already been verified via VerifyOTP.
func (s *BookingService) Complete(ctx context.Context, partnerID, bookingID string, in dto.CompleteBookingRequest) (dto.BookingResponse, error) {
	if err := validation.ValidateCompleteBooking(in); err != nil {
		return dto.BookingResponse{}, err
	}

	now := s.now()
	images := make([]model.BookingCompletionImage, 0, len(in.Images))
	for i, url := range in.Images {
		images = append(images, model.BookingCompletionImage{
			ID:        s.id(),
			BookingID: bookingID,
			Sequence:  i,
			URL:       url,
			CreatedAt: now,
		})
	}

	booking, err := s.repo.Complete(ctx, bookingID, partnerID, repository.BookingCompleteInput{
		Weight:     in.Weight,
		WeightUnit: strings.ToLower(strings.TrimSpace(in.WeightUnit)),
		AmountPaid: in.AmountPaid,
		Images:     images,
	})
	if err != nil {
		return dto.BookingResponse{}, err
	}
	s.notifications.BookingCompleted(ctx, booking)
	return s.toBookingResponse(booking), nil
}

func (s *BookingService) listStatusLogs(ctx context.Context, bookingID string) ([]dto.BookingStatusLogResponse, error) {
	logs, err := s.repo.ListStatusLogs(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.BookingStatusLogResponse, 0, len(logs))
	for _, l := range logs {
		out = append(out, dto.BookingStatusLogResponse{
			Status:    string(l.Status),
			CreatedAt: l.CreatedAt,
		})
	}
	return out, nil
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

// buildBookingImages converts uploaded image URLs into ordered, ID-assigned
// model rows for the given booking.
func buildBookingImages(bookingID string, urls []string, now time.Time, genID func() string) []model.BookingImage {
	images := make([]model.BookingImage, 0, len(urls))
	for i, url := range urls {
		images = append(images, model.BookingImage{
			ID:        genID(),
			BookingID: bookingID,
			Sequence:  i,
			URL:       url,
			CreatedAt: now,
		})
	}
	return images
}

// buildBookingImagesIfProvided returns nil (keep existing images unchanged)
// when urls is nil, otherwise builds the replacement image rows.
func buildBookingImagesIfProvided(bookingID string, urls []string, now time.Time, genID func() string) []model.BookingImage {
	if urls == nil {
		return nil
	}
	return buildBookingImages(bookingID, urls, now, genID)
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
	case string(model.BookingCompleted):
		return model.BookingCompleted, nil
	default:
		return "", utils.NewValidationError("status is invalid: must be one of pending, accepted, rejected, cancelled or completed")
	}
}

func (s *BookingService) toBookingPage(bookings []model.Booking, page, pageSize int, total int64) dto.PageResult[dto.BookingResponse] {
	out := make([]dto.BookingResponse, 0, len(bookings))
	for _, b := range bookings {
		out = append(out, s.toBookingResponse(b))
	}
	return dto.PageResult[dto.BookingResponse]{
		Data:       out,
		Pagination: dto.NewPagination(page, pageSize, total),
	}
}

func (s *BookingService) toBookingResponse(b model.Booking) dto.BookingResponse {
	images := make([]string, 0, len(b.Images))
	for _, img := range b.Images {
		images = append(images, resolveImageURL(s.baseURL, img.URL))
	}
	completionImages := make([]string, 0, len(b.CompletionImages))
	for _, img := range b.CompletionImages {
		completionImages = append(completionImages, resolveImageURL(s.baseURL, img.URL))
	}
	res := dto.BookingResponse{
		ID:              b.ID,
		Status:          string(b.Status),
		SlotDate:        b.SlotDate,
		SlotStartTime:   b.SlotStartTime,
		SlotEndTime:     b.SlotEndTime,
		PickupLatitude:  b.PickupLatitude,
		PickupLongitude: b.PickupLongitude,
		PickupAddress:   b.PickupAddress,
		Images:          images,
		Description:     b.Description,
		Note:            b.Note,
		Reason:          b.Reason,
		OTPVerified:     b.OTPVerifiedAt != nil,
		Weight:          b.Weight,
		WeightUnit:      b.WeightUnit,
		AmountPaid:      b.AmountPaid,
		CreatedAt:       b.CreatedAt,
		UpdatedAt:       b.UpdatedAt,
	}
	if len(completionImages) > 0 {
		res.CompletionImages = completionImages
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
