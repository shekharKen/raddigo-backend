package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/raddigo/raddigo/internal/model"
	"github.com/raddigo/raddigo/internal/utils"
)

// BookingRepository defines persistence operations for slot bookings.
type BookingRepository interface {
	UserExists(ctx context.Context, userID string) (bool, error)
	PartnerExists(ctx context.Context, partnerID string) (bool, error)
	Create(ctx context.Context, booking *model.Booking) error
	GetByID(ctx context.Context, id string) (model.Booking, error)
	ListByUser(ctx context.Context, userID string, status model.BookingStatus, limit, offset int) ([]model.Booking, int64, error)
	ListByPartner(ctx context.Context, partnerID string, status model.BookingStatus, limit, offset int) ([]model.Booking, int64, error)
	NextForUser(ctx context.Context, userID string) (model.Booking, error)
	NextForPartner(ctx context.Context, partnerID string) (model.Booking, error)
	SlotAccepted(ctx context.Context, partnerID, slotDate, slotStartTime string) (bool, error)
	Accept(ctx context.Context, bookingID, partnerID string) (model.Booking, error)
	Reject(ctx context.Context, bookingID, partnerID string) (model.Booking, error)
	Update(ctx context.Context, bookingID, userID string, in BookingUpdateInput) (model.Booking, error)
	Cancel(ctx context.Context, bookingID, userID string) (model.Booking, error)
	SendOTP(ctx context.Context, bookingID, partnerID, otp string, otpExpiry time.Time) (model.Booking, error)
	VerifyOTP(ctx context.Context, bookingID, partnerID, otp string, now time.Time) (model.Booking, error)
	Complete(ctx context.Context, bookingID, partnerID string, in BookingCompleteInput) (model.Booking, error)
	ListStatusLogs(ctx context.Context, bookingID string) ([]model.BookingStatusLog, error)
	ListPickupsDueForReminder(ctx context.Context, slotDate, fromTime, toTime string) ([]model.Booking, error)
	ClaimReminder(ctx context.Context, bookingID string, sentAt time.Time) (bool, error)
}

// BookingCompleteInput carries the completion details a partner submits to
// finish an accepted booking.
type BookingCompleteInput struct {
	Weight     float64
	WeightUnit string
	AmountPaid float64
	Images     []model.BookingCompletionImage
}

// BookingUpdateInput carries the mutable fields of a booking update. A nil
// Images means the existing images are kept; a non-nil Images wholesale
// replaces them.
type BookingUpdateInput struct {
	SlotDate        string
	SlotStartTime   string
	SlotEndTime     string
	PickupLatitude  float64
	PickupLongitude float64
	PickupAddress   string
	Images          []model.BookingImage
	Description     string
	Note            string
}

// GormBookingRepository is a GORM-backed BookingRepository.
type GormBookingRepository struct {
	db *gorm.DB
}

// NewGormBookingRepository creates a GormBookingRepository.
func NewGormBookingRepository(db *gorm.DB) *GormBookingRepository {
	return &GormBookingRepository{db: db}
}

// UserExists reports whether a user with the given id exists.
func (r *GormBookingRepository) UserExists(ctx context.Context, userID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", userID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("count user: %w", err)
	}
	return count > 0, nil
}

// PartnerExists reports whether a partner with the given id exists.
func (r *GormBookingRepository) PartnerExists(ctx context.Context, partnerID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.Partner{}).
		Where("id = ?", partnerID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("count partner: %w", err)
	}
	return count > 0, nil
}

// Create stores a new booking and its initial status log entry.
func (r *GormBookingRepository) Create(ctx context.Context, booking *model.Booking) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(booking).Error; err != nil {
			return fmt.Errorf("create booking: %w", err)
		}
		return insertBookingStatusLog(tx, booking.ID, booking.Status)
	})
}

// GetByID returns a booking by id with its user, partner and images preloaded.
func (r *GormBookingRepository) GetByID(ctx context.Context, id string) (model.Booking, error) {
	var booking model.Booking
	if err := r.db.WithContext(ctx).
		Preload("User").
		Preload("Partner").
		Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sequence ASC") }).
		Where("id = ?", id).
		First(&booking).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Booking{}, utils.ErrNotFound
		}
		return model.Booking{}, fmt.Errorf("get booking: %w", err)
	}
	return booking, nil
}

// ListByUser returns a paginated set of a user's bookings ordered by the
// nearest slot date/time first, each with its partner and images preloaded.
// An empty status returns all states.
func (r *GormBookingRepository) ListByUser(ctx context.Context, userID string, status model.BookingStatus, limit, offset int) ([]model.Booking, int64, error) {
	base := func(db *gorm.DB) *gorm.DB {
		db = db.Model(&model.Booking{}).Where("user_id = ?", userID)
		if status != "" {
			db = db.Where("status = ?", status)
		}
		return db
	}

	var total int64
	if err := base(r.db.WithContext(ctx)).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count user bookings: %w", err)
	}

	var bookings []model.Booking
	if err := base(r.db.WithContext(ctx)).
		Preload("Partner").
		Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sequence ASC") }).
		Order("slot_date ASC, slot_start_time ASC").
		Limit(limit).
		Offset(offset).
		Find(&bookings).Error; err != nil {
		return nil, 0, fmt.Errorf("list user bookings: %w", err)
	}
	return bookings, total, nil
}

// ListByPartner returns a paginated set of a partner's bookings ordered by the
// nearest slot date/time first, each with the requesting user and images
// preloaded. An empty status returns all states.
func (r *GormBookingRepository) ListByPartner(ctx context.Context, partnerID string, status model.BookingStatus, limit, offset int) ([]model.Booking, int64, error) {
	base := func(db *gorm.DB) *gorm.DB {
		db = db.Model(&model.Booking{}).Where("partner_id = ?", partnerID)
		if status != "" {
			db = db.Where("status = ?", status)
		}
		return db
	}

	var total int64
	if err := base(r.db.WithContext(ctx)).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count partner bookings: %w", err)
	}

	var bookings []model.Booking
	if err := base(r.db.WithContext(ctx)).
		Preload("User").
		Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sequence ASC") }).
		Order("slot_date ASC, slot_start_time ASC").
		Limit(limit).
		Offset(offset).
		Find(&bookings).Error; err != nil {
		return nil, 0, fmt.Errorf("list partner bookings: %w", err)
	}
	return bookings, total, nil
}

// NextForUser returns a user's next scheduled (accepted) pickup, the one with
// the nearest slot date/time, with its user, partner and images preloaded. It
// returns utils.ErrNotFound when the user has no accepted bookings.
func (r *GormBookingRepository) NextForUser(ctx context.Context, userID string) (model.Booking, error) {
	var booking model.Booking
	if err := r.db.WithContext(ctx).
		Preload("User").
		Preload("Partner").
		Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sequence ASC") }).
		Where("user_id = ? AND status = ?", userID, model.BookingAccepted).
		Order("slot_date ASC, slot_start_time ASC").
		First(&booking).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Booking{}, utils.ErrNotFound
		}
		return model.Booking{}, fmt.Errorf("next user booking: %w", err)
	}
	return booking, nil
}

// NextForPartner returns a partner's next scheduled (accepted) pickup, the
// one with the nearest slot date/time, with the requesting user, partner and
// images preloaded. It returns utils.ErrNotFound when the partner has no
// accepted bookings.
func (r *GormBookingRepository) NextForPartner(ctx context.Context, partnerID string) (model.Booking, error) {
	var booking model.Booking
	if err := r.db.WithContext(ctx).
		Preload("User").
		Preload("Partner").
		Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sequence ASC") }).
		Where("partner_id = ? AND status = ?", partnerID, model.BookingAccepted).
		Order("slot_date ASC, slot_start_time ASC").
		First(&booking).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Booking{}, utils.ErrNotFound
		}
		return model.Booking{}, fmt.Errorf("next partner booking: %w", err)
	}
	return booking, nil
}

// SlotAccepted reports whether the partner already has an accepted booking for
// the given date and start time (i.e. the slot is taken).
func (r *GormBookingRepository) SlotAccepted(ctx context.Context, partnerID, slotDate, slotStartTime string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.Booking{}).
		Where("partner_id = ? AND slot_date = ? AND slot_start_time = ? AND status = ?",
			partnerID, slotDate, slotStartTime, model.BookingAccepted).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("count accepted slot: %w", err)
	}
	return count > 0, nil
}

// Accept transitions a pending booking to accepted and rejects any other
// pending bookings competing for the same slot, all within a transaction. It
// returns utils.ErrNotFound when the booking does not belong to the partner,
// utils.ErrInvalidState when it is not pending, and utils.ErrSlotUnavailable
// when the slot has already been taken.
func (r *GormBookingRepository) Accept(ctx context.Context, bookingID, partnerID string) (model.Booking, error) {
	var accepted model.Booking
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var booking model.Booking
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND partner_id = ?", bookingID, partnerID).
			First(&booking).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return utils.ErrNotFound
			}
			return fmt.Errorf("load booking: %w", err)
		}
		if booking.Status != model.BookingPending {
			return utils.ErrInvalidState
		}

		var taken int64
		if err := tx.Model(&model.Booking{}).
			Where("partner_id = ? AND slot_date = ? AND slot_start_time = ? AND status = ?",
				partnerID, booking.SlotDate, booking.SlotStartTime, model.BookingAccepted).
			Count(&taken).Error; err != nil {
			return fmt.Errorf("count accepted slot: %w", err)
		}
		if taken > 0 {
			return utils.ErrSlotUnavailable
		}

		if err := tx.Model(&model.Booking{}).
			Where("id = ?", booking.ID).
			Update("status", model.BookingAccepted).Error; err != nil {
			return fmt.Errorf("accept booking: %w", err)
		}

		// Disable the slot for everyone else: reject other pending requests for
		// the same partner slot.
		if err := tx.Model(&model.Booking{}).
			Where("partner_id = ? AND slot_date = ? AND slot_start_time = ? AND status = ? AND id <> ?",
				partnerID, booking.SlotDate, booking.SlotStartTime, model.BookingPending, booking.ID).
			Update("status", model.BookingRejected).Error; err != nil {
			return fmt.Errorf("reject competing bookings: %w", err)
		}

		if err := insertBookingStatusLog(tx, booking.ID, model.BookingAccepted); err != nil {
			return err
		}

		if err := tx.Preload("User").Preload("Partner").
			Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sequence ASC") }).
			Where("id = ?", booking.ID).First(&accepted).Error; err != nil {
			return fmt.Errorf("reload booking: %w", err)
		}
		return nil
	})
	if err != nil {
		return model.Booking{}, err
	}
	return accepted, nil
}

// Reject transitions a pending booking to rejected. It returns
// utils.ErrNotFound when the booking does not belong to the partner and
// utils.ErrInvalidState when it is not pending.
func (r *GormBookingRepository) Reject(ctx context.Context, bookingID, partnerID string) (model.Booking, error) {
	var rejected model.Booking
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var booking model.Booking
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND partner_id = ?", bookingID, partnerID).
			First(&booking).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return utils.ErrNotFound
			}
			return fmt.Errorf("load booking: %w", err)
		}
		if booking.Status != model.BookingPending {
			return utils.ErrInvalidState
		}
		if err := tx.Model(&model.Booking{}).
			Where("id = ?", booking.ID).
			Update("status", model.BookingRejected).Error; err != nil {
			return fmt.Errorf("reject booking: %w", err)
		}
		if err := insertBookingStatusLog(tx, booking.ID, model.BookingRejected); err != nil {
			return err
		}
		if err := tx.Preload("User").Preload("Partner").
			Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sequence ASC") }).
			Where("id = ?", booking.ID).First(&rejected).Error; err != nil {
			return fmt.Errorf("reload booking: %w", err)
		}
		return nil
	})
	if err != nil {
		return model.Booking{}, err
	}
	return rejected, nil
}

// Update updates a user's pending booking with new slot and pickup details. It
// returns utils.ErrNotFound when the booking does not belong to the user,
// utils.ErrInvalidState when it is not pending, and utils.ErrSlotUnavailable
// when the requested slot has already been accepted for another booking.
func (r *GormBookingRepository) Update(ctx context.Context, bookingID, userID string, in BookingUpdateInput) (model.Booking, error) {
	var updated model.Booking
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var booking model.Booking
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", bookingID, userID).
			First(&booking).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return utils.ErrNotFound
			}
			return fmt.Errorf("load booking: %w", err)
		}
		if booking.Status != model.BookingPending {
			return utils.ErrInvalidState
		}

		var taken int64
		if err := tx.Model(&model.Booking{}).
			Where("partner_id = ? AND slot_date = ? AND slot_start_time = ? AND status = ? AND id <> ?",
				booking.PartnerID, in.SlotDate, in.SlotStartTime, model.BookingAccepted, booking.ID).
			Count(&taken).Error; err != nil {
			return fmt.Errorf("count accepted slot: %w", err)
		}
		if taken > 0 {
			return utils.ErrSlotUnavailable
		}

		fields := map[string]interface{}{
			"slot_date":        in.SlotDate,
			"slot_start_time":  in.SlotStartTime,
			"slot_end_time":    in.SlotEndTime,
			"pickup_latitude":  in.PickupLatitude,
			"pickup_longitude": in.PickupLongitude,
			"pickup_address":   in.PickupAddress,
			"description":      in.Description,
			"note":             in.Note,
		}
		if err := tx.Model(&model.Booking{}).
			Where("id = ?", booking.ID).
			Updates(fields).Error; err != nil {
			return fmt.Errorf("update booking: %w", err)
		}

		if in.Images != nil {
			if err := tx.Where("booking_id = ?", booking.ID).Delete(&model.BookingImage{}).Error; err != nil {
				return fmt.Errorf("delete booking images: %w", err)
			}
			if len(in.Images) > 0 {
				if err := tx.Create(&in.Images).Error; err != nil {
					return fmt.Errorf("create booking images: %w", err)
				}
			}
		}

		if err := tx.Preload("Partner").
			Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sequence ASC") }).
			Where("id = ?", booking.ID).First(&updated).Error; err != nil {
			return fmt.Errorf("reload booking: %w", err)
		}
		return nil
	})
	if err != nil {
		return model.Booking{}, err
	}
	return updated, nil
}

// Cancel transitions a pending or accepted booking owned by the user to
// cancelled. It returns utils.ErrNotFound when the booking does not belong to
// the user and utils.ErrInvalidState when it has already been rejected or
// cancelled.
func (r *GormBookingRepository) Cancel(ctx context.Context, bookingID, userID string) (model.Booking, error) {
	var cancelled model.Booking
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var booking model.Booking
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", bookingID, userID).
			First(&booking).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return utils.ErrNotFound
			}
			return fmt.Errorf("load booking: %w", err)
		}
		if booking.Status != model.BookingPending && booking.Status != model.BookingAccepted {
			return utils.ErrInvalidState
		}
		if err := tx.Model(&model.Booking{}).
			Where("id = ?", booking.ID).
			Update("status", model.BookingCancelled).Error; err != nil {
			return fmt.Errorf("cancel booking: %w", err)
		}
		if err := insertBookingStatusLog(tx, booking.ID, model.BookingCancelled); err != nil {
			return err
		}
		if err := tx.Preload("Partner").
			Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sequence ASC") }).
			Where("id = ?", booking.ID).First(&cancelled).Error; err != nil {
			return fmt.Errorf("reload booking: %w", err)
		}
		return nil
	})
	if err != nil {
		return model.Booking{}, err
	}
	return cancelled, nil
}

// SendOTP generates and stores a fresh completion OTP (with expiry) on an
// accepted booking for the caller to email to the customer. It returns
// utils.ErrNotFound when the booking does not belong to the partner and
// utils.ErrInvalidState when it is not accepted.
func (r *GormBookingRepository) SendOTP(ctx context.Context, bookingID, partnerID, otp string, otpExpiry time.Time) (model.Booking, error) {
	var updated model.Booking
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var booking model.Booking
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND partner_id = ?", bookingID, partnerID).
			First(&booking).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return utils.ErrNotFound
			}
			return fmt.Errorf("load booking: %w", err)
		}
		if booking.Status != model.BookingAccepted {
			return utils.ErrInvalidState
		}
		fields := map[string]interface{}{
			"completion_otp":        otp,
			"completion_otp_expiry": otpExpiry,
			"otp_verified_at":       nil,
		}
		if err := tx.Model(&model.Booking{}).
			Where("id = ?", booking.ID).
			Updates(fields).Error; err != nil {
			return fmt.Errorf("send otp: %w", err)
		}
		if err := tx.Preload("User").Preload("Partner").
			Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sequence ASC") }).
			Where("id = ?", booking.ID).First(&updated).Error; err != nil {
			return fmt.Errorf("reload booking: %w", err)
		}
		return nil
	})
	if err != nil {
		return model.Booking{}, err
	}
	return updated, nil
}

// VerifyOTP confirms the completion OTP a partner reads back from the
// customer, marking it verified so the booking can then be completed. It
// returns utils.ErrNotFound when the booking does not belong to the partner,
// utils.ErrInvalidState when it is not accepted, and utils.ErrInvalidOTP
// when the OTP is wrong, missing or expired. Already-verified bookings are a
// no-op success.
func (r *GormBookingRepository) VerifyOTP(ctx context.Context, bookingID, partnerID, otp string, now time.Time) (model.Booking, error) {
	var verified model.Booking
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var booking model.Booking
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND partner_id = ?", bookingID, partnerID).
			First(&booking).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return utils.ErrNotFound
			}
			return fmt.Errorf("load booking: %w", err)
		}
		if booking.Status != model.BookingAccepted {
			return utils.ErrInvalidState
		}
		if booking.OTPVerifiedAt == nil {
			if booking.CompletionOTP == "" || booking.CompletionOTP != otp {
				return utils.ErrInvalidOTP
			}
			if booking.CompletionOTPExpiry.IsZero() || now.After(booking.CompletionOTPExpiry) {
				return utils.ErrInvalidOTP
			}
			if err := tx.Model(&model.Booking{}).
				Where("id = ?", booking.ID).
				Update("otp_verified_at", now).Error; err != nil {
				return fmt.Errorf("verify otp: %w", err)
			}
		}
		if err := tx.Preload("User").Preload("Partner").
			Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sequence ASC") }).
			Where("id = ?", booking.ID).First(&verified).Error; err != nil {
			return fmt.Errorf("reload booking: %w", err)
		}
		return nil
	})
	if err != nil {
		return model.Booking{}, err
	}
	return verified, nil
}

// Complete transitions an accepted booking to completed, storing the
// weight, amount paid and scrap images the partner submits. It returns
// utils.ErrNotFound when the booking does not belong to the partner,
// utils.ErrInvalidState when it is not accepted, and
// utils.ErrOTPNotVerified when the completion OTP hasn't been verified yet.
func (r *GormBookingRepository) Complete(ctx context.Context, bookingID, partnerID string, in BookingCompleteInput) (model.Booking, error) {
	var updated model.Booking
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var booking model.Booking
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND partner_id = ?", bookingID, partnerID).
			First(&booking).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return utils.ErrNotFound
			}
			return fmt.Errorf("load booking: %w", err)
		}
		if booking.Status != model.BookingAccepted {
			return utils.ErrInvalidState
		}
		if booking.OTPVerifiedAt == nil {
			return utils.ErrOTPNotVerified
		}
		fields := map[string]interface{}{
			"status":      model.BookingCompleted,
			"weight":      in.Weight,
			"weight_unit": in.WeightUnit,
			"amount_paid": in.AmountPaid,
		}
		if err := tx.Model(&model.Booking{}).
			Where("id = ?", booking.ID).
			Updates(fields).Error; err != nil {
			return fmt.Errorf("complete booking: %w", err)
		}
		if len(in.Images) > 0 {
			if err := tx.Create(&in.Images).Error; err != nil {
				return fmt.Errorf("create completion images: %w", err)
			}
		}
		if err := insertBookingStatusLog(tx, booking.ID, model.BookingCompleted); err != nil {
			return err
		}
		if err := tx.Preload("User").Preload("Partner").
			Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sequence ASC") }).
			Preload("CompletionImages", func(db *gorm.DB) *gorm.DB { return db.Order("sequence ASC") }).
			Where("id = ?", booking.ID).First(&updated).Error; err != nil {
			return fmt.Errorf("reload booking: %w", err)
		}
		return nil
	})
	if err != nil {
		return model.Booking{}, err
	}
	return updated, nil
}

// ListStatusLogs returns a booking's status transition history, oldest first.
func (r *GormBookingRepository) ListStatusLogs(ctx context.Context, bookingID string) ([]model.BookingStatusLog, error) {
	var logs []model.BookingStatusLog
	if err := r.db.WithContext(ctx).
		Where("booking_id = ?", bookingID).
		Order("created_at ASC").
		Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("list booking status logs: %w", err)
	}
	return logs, nil
}

// ListPickupsDueForReminder returns accepted bookings for slotDate whose
// slot_start_time falls within [fromTime, toTime] (both "HH:MM", inclusive)
// and that have not yet had a pickup reminder sent, with user and partner
// preloaded so a notification can be sent immediately.
func (r *GormBookingRepository) ListPickupsDueForReminder(ctx context.Context, slotDate, fromTime, toTime string) ([]model.Booking, error) {
	var bookings []model.Booking
	if err := r.db.WithContext(ctx).
		Preload("User").
		Preload("Partner").
		Where("status = ? AND slot_date = ? AND slot_start_time BETWEEN ? AND ? AND reminder_sent_at IS NULL",
			model.BookingAccepted, slotDate, fromTime, toTime).
		Find(&bookings).Error; err != nil {
		return nil, fmt.Errorf("list pickups due for reminder: %w", err)
	}
	return bookings, nil
}

// ClaimReminder atomically marks a booking's pickup reminder as sent,
// returning false when it was already claimed by a previous poll (i.e. the
// caller should not send a duplicate notification).
func (r *GormBookingRepository) ClaimReminder(ctx context.Context, bookingID string, sentAt time.Time) (bool, error) {
	res := r.db.WithContext(ctx).
		Model(&model.Booking{}).
		Where("id = ? AND reminder_sent_at IS NULL", bookingID).
		Update("reminder_sent_at", sentAt)
	if res.Error != nil {
		return false, fmt.Errorf("claim reminder: %w", res.Error)
	}
	return res.RowsAffected == 1, nil
}

// insertBookingStatusLog records a booking status transition within tx.
func insertBookingStatusLog(tx *gorm.DB, bookingID string, status model.BookingStatus) error {
	log := model.BookingStatusLog{
		ID:        uuid.NewString(),
		BookingID: bookingID,
		Status:    status,
		CreatedAt: time.Now(),
	}
	if err := tx.Create(&log).Error; err != nil {
		return fmt.Errorf("create booking status log: %w", err)
	}
	return nil
}
