package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/raddigo/raddigo/internal/model"
	"github.com/raddigo/raddigo/internal/utils"
)

// PartnerRepository defines persistence operations for partners.
type PartnerRepository interface {
	Create(ctx context.Context, partner *model.Partner) error
	EmailExists(ctx context.Context, email string) (bool, error)
	GetByID(ctx context.Context, id string) (model.Partner, error)
	GetByEmail(ctx context.Context, email string) (model.Partner, error)
	GetByVerifyToken(ctx context.Context, token string) (model.Partner, error)
	MarkEmailVerified(ctx context.Context, id string) error
	UpdateProfile(ctx context.Context, id string, fields map[string]any) error
	SetResetToken(ctx context.Context, id, token string, expiry time.Time) error
	GetByResetToken(ctx context.Context, token string) (model.Partner, error)
	UpdatePassword(ctx context.Context, id, hashedPassword string) error
	SearchByLocation(ctx context.Context, lat, lng float64, limit, offset int) ([]model.Partner, int64, error)
}

// GormPartnerRepository is a GORM-backed PartnerRepository.
type GormPartnerRepository struct {
	db *gorm.DB
}

// NewGormPartnerRepository creates a GormPartnerRepository.
func NewGormPartnerRepository(db *gorm.DB) *GormPartnerRepository {
	return &GormPartnerRepository{db: db}
}

// Create stores a new partner together with its operating-area polygon and
// derives the indexed geography(Polygon) column used for spatial search.
func (r *GormPartnerRepository) Create(ctx context.Context, partner *model.Partner) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(partner).Error; err != nil {
			return fmt.Errorf("create partner: %w", err)
		}
		wkt := polygonWKT(partner.ServiceArea)
		if wkt == "" {
			return nil
		}
		if err := tx.Exec(
			`UPDATE partners SET service_area = ST_GeogFromText(?) WHERE id = ?`,
			wkt, partner.ID,
		).Error; err != nil {
			return fmt.Errorf("set service area: %w", err)
		}
		return nil
	})
}

// SearchByLocation returns a paginated set of verified partners whose operating-
// area polygon covers the given point. ST_Covers uses the GiST index on
// service_area for an index-backed bounding-box scan.
func (r *GormPartnerRepository) SearchByLocation(ctx context.Context, lat, lng float64, limit, offset int) ([]model.Partner, int64, error) {
	cond := func(db *gorm.DB) *gorm.DB {
		return db.Model(&model.Partner{}).
			Where("email_verified = ?", true).
			Where("service_area IS NOT NULL").
			Where("ST_Covers(service_area, ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography)", lng, lat)
	}

	var total int64
	if err := cond(r.db.WithContext(ctx)).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count partners by location: %w", err)
	}

	var partners []model.Partner
	if err := cond(r.db.WithContext(ctx)).
		Preload("StoreAddress").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&partners).Error; err != nil {
		return nil, 0, fmt.Errorf("search partners by location: %w", err)
	}
	return partners, total, nil
}

// polygonWKT builds a closed WKT polygon ring (SRID 4326) from ordered points.
// Coordinates are numeric floats, so the produced string is injection-safe.
func polygonWKT(points []model.PolygonPoint) string {
	if len(points) < 3 {
		return ""
	}
	coords := make([]string, 0, len(points)+1)
	for _, p := range points {
		coords = append(coords, coordPair(p.Longitude, p.Latitude))
	}
	// Close the ring if the last point does not match the first.
	first, last := points[0], points[len(points)-1]
	if first.Longitude != last.Longitude || first.Latitude != last.Latitude {
		coords = append(coords, coordPair(first.Longitude, first.Latitude))
	}
	return fmt.Sprintf("SRID=4326;POLYGON((%s))", strings.Join(coords, ", "))
}

func coordPair(lng, lat float64) string {
	return strconv.FormatFloat(lng, 'f', -1, 64) + " " + strconv.FormatFloat(lat, 'f', -1, 64)
}

// EmailExists reports whether a partner with the given email already exists.
func (r *GormPartnerRepository) EmailExists(ctx context.Context, email string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.Partner{}).
		Where("email = ?", email).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("count partners: %w", err)
	}
	return count > 0, nil
}

// GetByEmail returns the partner with the given email, or ErrNotFound.
func (r *GormPartnerRepository) GetByEmail(ctx context.Context, email string) (model.Partner, error) {
	var partner model.Partner
	if err := r.db.WithContext(ctx).
		First(&partner, "email = ?", strings.ToLower(strings.TrimSpace(email))).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Partner{}, utils.ErrNotFound
		}
		return model.Partner{}, fmt.Errorf("get partner by email: %w", err)
	}
	return partner, nil
}

// GetByVerifyToken returns the partner matching the verification token, or
// ErrNotFound if none matches.
func (r *GormPartnerRepository) GetByVerifyToken(ctx context.Context, token string) (model.Partner, error) {
	var partner model.Partner
	if err := r.db.WithContext(ctx).
		Preload("ServiceArea").
		Preload("StoreAddress").
		First(&partner, "verify_token = ?", token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Partner{}, utils.ErrNotFound
		}
		return model.Partner{}, fmt.Errorf("get partner by token: %w", err)
	}
	return partner, nil
}

// MarkEmailVerified flags the partner as verified and clears the token.
func (r *GormPartnerRepository) MarkEmailVerified(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).
		Model(&model.Partner{}).
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

// GetByID returns the partner with the given id, preloading its store address
// and operating-area polygon, or ErrNotFound.
func (r *GormPartnerRepository) GetByID(ctx context.Context, id string) (model.Partner, error) {
	var partner model.Partner
	if err := r.db.WithContext(ctx).
		Preload("ServiceArea").
		Preload("StoreAddress").
		First(&partner, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Partner{}, utils.ErrNotFound
		}
		return model.Partner{}, fmt.Errorf("get partner by id: %w", err)
	}
	return partner, nil
}

// UpdateProfile updates the given profile fields for the partner.
func (r *GormPartnerRepository) UpdateProfile(ctx context.Context, id string, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	res := r.db.WithContext(ctx).
		Model(&model.Partner{}).
		Where("id = ?", id).
		Updates(fields)
	if res.Error != nil {
		return fmt.Errorf("update partner profile: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return utils.ErrNotFound
	}
	return nil
}

// SetResetToken stores a password reset token and its expiry for the partner.
func (r *GormPartnerRepository) SetResetToken(ctx context.Context, id, token string, expiry time.Time) error {
	res := r.db.WithContext(ctx).
		Model(&model.Partner{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"reset_token":        token,
			"reset_token_expiry": expiry,
		})
	if res.Error != nil {
		return fmt.Errorf("set reset token: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return utils.ErrNotFound
	}
	return nil
}

// GetByResetToken returns the partner matching the reset token, or ErrNotFound.
func (r *GormPartnerRepository) GetByResetToken(ctx context.Context, token string) (model.Partner, error) {
	var partner model.Partner
	if err := r.db.WithContext(ctx).
		First(&partner, "reset_token = ?", token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Partner{}, utils.ErrNotFound
		}
		return model.Partner{}, fmt.Errorf("get partner by reset token: %w", err)
	}
	return partner, nil
}

// UpdatePassword sets a new password hash and clears the reset token.
func (r *GormPartnerRepository) UpdatePassword(ctx context.Context, id, hashedPassword string) error {
	res := r.db.WithContext(ctx).
		Model(&model.Partner{}).
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
