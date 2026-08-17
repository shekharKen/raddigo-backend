package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/raddigo/raddigo/internal/model"
	"github.com/raddigo/raddigo/internal/utils"
)

// UserRepository defines persistence operations for users.
type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	EmailExists(ctx context.Context, email string) (bool, error)
	GetByEmail(ctx context.Context, email string) (model.User, error)
	GetByVerifyToken(ctx context.Context, token string) (model.User, error)
	MarkEmailVerified(ctx context.Context, id string) error
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

// GetByVerifyToken returns the user matching the verification token, or
// ErrNotFound if none matches.
func (r *GormUserRepository) GetByVerifyToken(ctx context.Context, token string) (model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).
		Preload("Addresses").
		First(&user, "verify_token = ?", token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.User{}, utils.ErrNotFound
		}
		return model.User{}, fmt.Errorf("get user by token: %w", err)
	}
	return user, nil
}

// MarkEmailVerified flags the user as verified and clears the token.
func (r *GormUserRepository) MarkEmailVerified(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"email_verified": true,
			"verify_token":   "",
		})
	if res.Error != nil {
		return fmt.Errorf("mark verified: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return utils.ErrNotFound
	}
	return nil
}
