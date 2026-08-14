package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/raddigo/raddigo/internal/model"
	"github.com/raddigo/raddigo/internal/utils"
)

// RagmanRepository defines persistence operations for ragmen.
type RagmanRepository interface {
	Create(ctx context.Context, ragman *model.Ragman) error
	EmailExists(ctx context.Context, email string) (bool, error)
	GetByVerifyToken(ctx context.Context, token string) (model.Ragman, error)
	MarkEmailVerified(ctx context.Context, id string) error
	SearchByLocation(ctx context.Context, lat, lng float64, limit, offset int) ([]model.Ragman, int64, error)
}

// GormRagmanRepository is a GORM-backed RagmanRepository.
type GormRagmanRepository struct {
	db *gorm.DB
}

// NewGormRagmanRepository creates a GormRagmanRepository.
func NewGormRagmanRepository(db *gorm.DB) *GormRagmanRepository {
	return &GormRagmanRepository{db: db}
}

// Create stores a new ragman together with its operating-area polygon and
// derives the indexed geography(Polygon) column used for spatial search.
func (r *GormRagmanRepository) Create(ctx context.Context, ragman *model.Ragman) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(ragman).Error; err != nil {
			return fmt.Errorf("create ragman: %w", err)
		}
		wkt := polygonWKT(ragman.ServiceArea)
		if wkt == "" {
			return nil
		}
		if err := tx.Exec(
			`UPDATE ragmen SET service_area = ST_GeogFromText(?) WHERE id = ?`,
			wkt, ragman.ID,
		).Error; err != nil {
			return fmt.Errorf("set service area: %w", err)
		}
		return nil
	})
}

// SearchByLocation returns a paginated set of verified ragmen whose operating-
// area polygon covers the given point. ST_Covers uses the GiST index on
// service_area for an index-backed bounding-box scan.
func (r *GormRagmanRepository) SearchByLocation(ctx context.Context, lat, lng float64, limit, offset int) ([]model.Ragman, int64, error) {
	cond := func(db *gorm.DB) *gorm.DB {
		return db.Model(&model.Ragman{}).
			Where("email_verified = ?", true).
			Where("service_area IS NOT NULL").
			Where("ST_Covers(service_area, ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography)", lng, lat)
	}

	var total int64
	if err := cond(r.db.WithContext(ctx)).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count ragmen by location: %w", err)
	}

	var ragmen []model.Ragman
	if err := cond(r.db.WithContext(ctx)).
		Preload("StoreAddress").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&ragmen).Error; err != nil {
		return nil, 0, fmt.Errorf("search ragmen by location: %w", err)
	}
	return ragmen, total, nil
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

// EmailExists reports whether a ragman with the given email already exists.
func (r *GormRagmanRepository) EmailExists(ctx context.Context, email string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.Ragman{}).
		Where("email = ?", email).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("count ragmen: %w", err)
	}
	return count > 0, nil
}

// GetByVerifyToken returns the ragman matching the verification token, or
// ErrNotFound if none matches.
func (r *GormRagmanRepository) GetByVerifyToken(ctx context.Context, token string) (model.Ragman, error) {
	var ragman model.Ragman
	if err := r.db.WithContext(ctx).
		Preload("ServiceArea").
		Preload("StoreAddress").
		First(&ragman, "verify_token = ?", token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Ragman{}, utils.ErrNotFound
		}
		return model.Ragman{}, fmt.Errorf("get ragman by token: %w", err)
	}
	return ragman, nil
}

// MarkEmailVerified flags the ragman as verified and clears the token.
func (r *GormRagmanRepository) MarkEmailVerified(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).
		Model(&model.Ragman{}).
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
