package repository

import (
	"context"
	"errors"
	"fmt"

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
	ListByUser(ctx context.Context, userID string, limit, offset int) ([]model.Booking, int64, error)
	ListByPartner(ctx context.Context, partnerID string, status model.BookingStatus, limit, offset int) ([]model.Booking, int64, error)
	SlotAccepted(ctx context.Context, partnerID, slotDate, slotStartTime string) (bool, error)
	Accept(ctx context.Context, bookingID, partnerID string) (model.Booking, error)
	Reject(ctx context.Context, bookingID, partnerID string) (model.Booking, error)
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

// Create stores a new booking.
func (r *GormBookingRepository) Create(ctx context.Context, booking *model.Booking) error {
	if err := r.db.WithContext(ctx).Create(booking).Error; err != nil {
		return fmt.Errorf("create booking: %w", err)
	}
	return nil
}

// GetByID returns a booking by id with its user and partner preloaded.
func (r *GormBookingRepository) GetByID(ctx context.Context, id string) (model.Booking, error) {
	var booking model.Booking
	if err := r.db.WithContext(ctx).
		Preload("User").
		Preload("Partner").
		Where("id = ?", id).
		First(&booking).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Booking{}, utils.ErrNotFound
		}
		return model.Booking{}, fmt.Errorf("get booking: %w", err)
	}
	return booking, nil
}

// ListByUser returns a paginated set of a user's bookings, newest first, each
// with its partner preloaded.
func (r *GormBookingRepository) ListByUser(ctx context.Context, userID string, limit, offset int) ([]model.Booking, int64, error) {
	base := func(db *gorm.DB) *gorm.DB {
		return db.Model(&model.Booking{}).Where("user_id = ?", userID)
	}

	var total int64
	if err := base(r.db.WithContext(ctx)).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count user bookings: %w", err)
	}

	var bookings []model.Booking
	if err := base(r.db.WithContext(ctx)).
		Preload("Partner").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&bookings).Error; err != nil {
		return nil, 0, fmt.Errorf("list user bookings: %w", err)
	}
	return bookings, total, nil
}

// ListByPartner returns a paginated set of a partner's bookings, newest first,
// each with the requesting user preloaded. An empty status returns all states.
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
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&bookings).Error; err != nil {
		return nil, 0, fmt.Errorf("list partner bookings: %w", err)
	}
	return bookings, total, nil
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

		if err := tx.Preload("User").Preload("Partner").
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
		if err := tx.Preload("User").Preload("Partner").
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
