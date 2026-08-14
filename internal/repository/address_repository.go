package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/raddigo/raddigo/internal/model"
	"github.com/raddigo/raddigo/internal/utils"
)

// AddressRepository defines persistence operations for user addresses.
type AddressRepository interface {
	UserExists(ctx context.Context, userID string) (bool, error)
	Create(ctx context.Context, address *model.Address) error
	ListByUser(ctx context.Context, userID string, limit, offset int) ([]model.Address, int64, error)
	GetByID(ctx context.Context, userID, id string) (model.Address, error)
	Update(ctx context.Context, address *model.Address) error
	Delete(ctx context.Context, userID, id string) error
}

// GormAddressRepository is a GORM-backed AddressRepository.
type GormAddressRepository struct {
	db *gorm.DB
}

// NewGormAddressRepository creates a GormAddressRepository.
func NewGormAddressRepository(db *gorm.DB) *GormAddressRepository {
	return &GormAddressRepository{db: db}
}

// UserExists reports whether a user with the given id exists.
func (r *GormAddressRepository) UserExists(ctx context.Context, userID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", userID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("count user: %w", err)
	}
	return count > 0, nil
}

// Create stores a new address.
func (r *GormAddressRepository) Create(ctx context.Context, address *model.Address) error {
	if err := r.db.WithContext(ctx).Create(address).Error; err != nil {
		return fmt.Errorf("create address: %w", err)
	}
	return nil
}

// ListByUser returns a paginated set of addresses belonging to a user, newest
// first, along with the total count.
func (r *GormAddressRepository) ListByUser(ctx context.Context, userID string, limit, offset int) ([]model.Address, int64, error) {
	cond := func(db *gorm.DB) *gorm.DB {
		return db.Model(&model.Address{}).Where("user_id = ?", userID)
	}

	var total int64
	if err := cond(r.db.WithContext(ctx)).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count addresses: %w", err)
	}

	var addresses []model.Address
	if err := cond(r.db.WithContext(ctx)).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&addresses).Error; err != nil {
		return nil, 0, fmt.Errorf("list addresses: %w", err)
	}
	return addresses, total, nil
}

// GetByID returns a single address owned by the user, or ErrNotFound.
func (r *GormAddressRepository) GetByID(ctx context.Context, userID, id string) (model.Address, error) {
	var address model.Address
	if err := r.db.WithContext(ctx).
		First(&address, "id = ? AND user_id = ?", id, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Address{}, utils.ErrNotFound
		}
		return model.Address{}, fmt.Errorf("get address: %w", err)
	}
	return address, nil
}

// Update persists all columns of an existing address.
func (r *GormAddressRepository) Update(ctx context.Context, address *model.Address) error {
	if err := r.db.WithContext(ctx).Save(address).Error; err != nil {
		return fmt.Errorf("update address: %w", err)
	}
	return nil
}

// Delete removes an address owned by the user, or returns ErrNotFound.
func (r *GormAddressRepository) Delete(ctx context.Context, userID, id string) error {
	res := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&model.Address{})
	if res.Error != nil {
		return fmt.Errorf("delete address: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return utils.ErrNotFound
	}
	return nil
}
