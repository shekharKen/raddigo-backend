package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/raddigo/raddigo/internal/model"
	"github.com/raddigo/raddigo/internal/utils"
)

// UserRepository defines persistence operations for users.
type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	EmailExists(ctx context.Context, email string) (bool, error)
	GetByID(ctx context.Context, id string) (model.User, error)
	GetByEmail(ctx context.Context, email string) (model.User, error)
	MarkEmailVerified(ctx context.Context, id string) error
	UpdateProfile(ctx context.Context, id string, fields map[string]any) error
	UpdateFCMToken(ctx context.Context, id, token string) error
	UpsertPrimaryAddress(ctx context.Context, userID string, address *model.Address) error
	SetResetOTP(ctx context.Context, id, otp string, expiry time.Time) error
	SetResetToken(ctx context.Context, id, token string, expiry time.Time) error
	GetByResetToken(ctx context.Context, token string) (model.User, error)
	UpdatePassword(ctx context.Context, id, hashedPassword string) error
}

// GormUserRepository is a GORM-backed UserRepository.
type GormUserRepository struct {
	db *gorm.DB
}

// NewGormUserRepository creates a GormUserRepository.
func NewGormUserRepository(db *gorm.DB) *GormUserRepository {
	return &GormUserRepository{db: db}
}

// Create stores a new user together with its addresses.
func (r *GormUserRepository) Create(ctx context.Context, user *model.User) error {
	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// EmailExists reports whether a user with the given email already exists.
func (r *GormUserRepository) EmailExists(ctx context.Context, email string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("email = ?", email).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("count users: %w", err)
	}
	return count > 0, nil
}

// GetByID returns the user with the given id (with addresses), or ErrNotFound.
func (r *GormUserRepository) GetByID(ctx context.Context, id string) (model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).
		Preload("Addresses").
		First(&user, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.User{}, utils.ErrNotFound
		}
		return model.User{}, fmt.Errorf("get user by id: %w", err)
	}
	return user, nil
}

// GetByEmail returns the user with the given email, or ErrNotFound.
func (r *GormUserRepository) GetByEmail(ctx context.Context, email string) (model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).
		First(&user, "email = ?", strings.ToLower(strings.TrimSpace(email))).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.User{}, utils.ErrNotFound
		}
		return model.User{}, fmt.Errorf("get user by email: %w", err)
	}
	return user, nil
}

// MarkEmailVerified flags the user as verified and clears the OTP.
func (r *GormUserRepository) MarkEmailVerified(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"email_verified":    true,
			"verify_otp":        "",
			"verify_otp_expiry": time.Time{},
		})
	if res.Error != nil {
		return fmt.Errorf("mark verified: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return utils.ErrNotFound
	}
	return nil
}

// UpdateProfile updates the given profile fields for the user.
func (r *GormUserRepository) UpdateProfile(ctx context.Context, id string, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	res := r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", id).
		Updates(fields)
	if res.Error != nil {
		return fmt.Errorf("update user profile: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return utils.ErrNotFound
	}
	return nil
}

// UpdateFCMToken stores the device token used to push notifications to a user.
func (r *GormUserRepository) UpdateFCMToken(ctx context.Context, id, token string) error {
	res := r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", id).
		Update("fcm_token", token)
	if res.Error != nil {
		return fmt.Errorf("update user fcm token: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return utils.ErrNotFound
	}
	return nil
}

// UpsertPrimaryAddress updates the user's existing (oldest) address, or creates
// one when the user has none yet.
func (r *GormUserRepository) UpsertPrimaryAddress(ctx context.Context, userID string, address *model.Address) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.Address
		err := tx.Where("user_id = ?", userID).Order("created_at ASC").First(&existing).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if err := tx.Create(address).Error; err != nil {
					return fmt.Errorf("create user address: %w", err)
				}
				return nil
			}
			return fmt.Errorf("get user address: %w", err)
		}
		if err := tx.Model(&model.Address{}).
			Where("id = ?", existing.ID).
			Updates(addressUpdateFields(address)).Error; err != nil {
			return fmt.Errorf("update user address: %w", err)
		}
		return nil
	})
}

// SetResetOTP stores a password-reset OTP and its expiry for the user.
func (r *GormUserRepository) SetResetOTP(ctx context.Context, id, otp string, expiry time.Time) error {
	res := r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"reset_otp":        otp,
			"reset_otp_expiry": expiry,
		})
	if res.Error != nil {
		return fmt.Errorf("set reset otp: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return utils.ErrNotFound
	}
	return nil
}

// SetResetToken stores a password reset token and its expiry for the user,
// consuming (clearing) any pending reset OTP.
func (r *GormUserRepository) SetResetToken(ctx context.Context, id, token string, expiry time.Time) error {
	res := r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"reset_token":        token,
			"reset_token_expiry": expiry,
			"reset_otp":          "",
			"reset_otp_expiry":   time.Time{},
		})
	if res.Error != nil {
		return fmt.Errorf("set reset token: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return utils.ErrNotFound
	}
	return nil
}

// GetByResetToken returns the user matching the reset token, or ErrNotFound.
func (r *GormUserRepository) GetByResetToken(ctx context.Context, token string) (model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).
		First(&user, "reset_token = ?", token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.User{}, utils.ErrNotFound
		}
		return model.User{}, fmt.Errorf("get user by reset token: %w", err)
	}
	return user, nil
}

// UpdatePassword sets a new password hash and clears the reset token.
func (r *GormUserRepository) UpdatePassword(ctx context.Context, id, hashedPassword string) error {
	res := r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"password":           hashedPassword,
			"reset_token":        "",
			"reset_token_expiry": time.Time{},
		})
	if res.Error != nil {
		return fmt.Errorf("update password: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return utils.ErrNotFound
	}
	return nil
}
