package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/raddigo/raddigo/internal/model"
)

// RatingFilter narrows a rating query by direction and the involved parties.
// Empty UserID or PartnerID fields are ignored.
type RatingFilter struct {
	Direction model.RatingDirection
	UserID    string
	PartnerID string
}

// RatingStats holds aggregate rating figures for a single ratee.
type RatingStats struct {
	Average float64
	Total   int64
}

// PartnerRatingRow is a rating a partner received, enriched with the rater's name.
type PartnerRatingRow struct {
	model.Rating
	RaterFirstName string
	RaterLastName  string
}

// RatingRepository defines persistence operations for user/partner ratings.
type RatingRepository interface {
	UserExists(ctx context.Context, userID string) (bool, error)
	PartnerExists(ctx context.Context, partnerID string) (bool, error)
	Upsert(ctx context.Context, rating *model.Rating) error
	List(ctx context.Context, f RatingFilter, limit, offset int) ([]model.Rating, int64, error)
	ListReceivedByPartner(ctx context.Context, partnerID string, limit, offset int) ([]PartnerRatingRow, int64, error)
	Summary(ctx context.Context, f RatingFilter) (avg float64, total int64, err error)
	SummaryByPartnerIDs(ctx context.Context, partnerIDs []string) (map[string]RatingStats, error)
}

// GormRatingRepository is a GORM-backed RatingRepository.
type GormRatingRepository struct {
	db *gorm.DB
}

// NewGormRatingRepository creates a GormRatingRepository.
func NewGormRatingRepository(db *gorm.DB) *GormRatingRepository {
	return &GormRatingRepository{db: db}
}

// UserExists reports whether a user with the given id exists.
func (r *GormRatingRepository) UserExists(ctx context.Context, userID string) (bool, error) {
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
func (r *GormRatingRepository) PartnerExists(ctx context.Context, partnerID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.Partner{}).
		Where("id = ?", partnerID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("count partner: %w", err)
	}
	return count > 0, nil
}

// Upsert stores a rating, replacing any existing score/feedback for the same
// (user, partner, direction) so each side keeps a single current rating.
func (r *GormRatingRepository) Upsert(ctx context.Context, rating *model.Rating) error {
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "partner_id"}, {Name: "direction"}},
			DoUpdates: clause.AssignmentColumns([]string{"score", "feedback", "updated_at"}),
		}).
		Create(rating).Error; err != nil {
		return fmt.Errorf("upsert rating: %w", err)
	}
	return nil
}

// List returns a paginated set of ratings matching the filter, newest first,
// along with the total count.
func (r *GormRatingRepository) List(ctx context.Context, f RatingFilter, limit, offset int) ([]model.Rating, int64, error) {
	scope := r.scope(f)

	var total int64
	if err := scope(r.db.WithContext(ctx)).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count ratings: %w", err)
	}

	var ratings []model.Rating
	if err := scope(r.db.WithContext(ctx)).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&ratings).Error; err != nil {
		return nil, 0, fmt.Errorf("list ratings: %w", err)
	}
	return ratings, total, nil
}

// ListReceivedByPartner returns a paginated set of ratings a partner received
// from users, newest first, each joined with the rating user's name.
func (r *GormRatingRepository) ListReceivedByPartner(ctx context.Context, partnerID string, limit, offset int) ([]PartnerRatingRow, int64, error) {
	base := func(db *gorm.DB) *gorm.DB {
		return db.Model(&model.Rating{}).
			Where("ratings.direction = ?", model.RatingUserToPartner).
			Where("ratings.partner_id = ?", partnerID)
	}

	var total int64
	if err := base(r.db.WithContext(ctx)).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count partner ratings: %w", err)
	}

	var rows []PartnerRatingRow
	if err := base(r.db.WithContext(ctx)).
		Select("ratings.*, users.first_name AS rater_first_name, users.last_name AS rater_last_name").
		Joins("JOIN users ON users.id = ratings.user_id").
		Order("ratings.created_at DESC").
		Limit(limit).
		Offset(offset).
		Scan(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("list partner ratings: %w", err)
	}
	return rows, total, nil
}

// Summary returns the average score and total number of ratings matching the
// filter. The average is 0 when there are no ratings.
func (r *GormRatingRepository) Summary(ctx context.Context, f RatingFilter) (float64, int64, error) {
	var res struct {
		Average float64
		Total   int64
	}
	if err := r.scope(f)(r.db.WithContext(ctx)).
		Select("COALESCE(AVG(score), 0) AS average, COUNT(*) AS total").
		Scan(&res).Error; err != nil {
		return 0, 0, fmt.Errorf("summarize ratings: %w", err)
	}
	return res.Average, res.Total, nil
}

// SummaryByPartnerIDs returns rating aggregates (from users) keyed by partner
// id for the given partners. Partners without ratings are omitted.
func (r *GormRatingRepository) SummaryByPartnerIDs(ctx context.Context, partnerIDs []string) (map[string]RatingStats, error) {
	out := make(map[string]RatingStats, len(partnerIDs))
	if len(partnerIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		PartnerID string
		Average   float64
		Total     int64
	}
	if err := r.db.WithContext(ctx).
		Model(&model.Rating{}).
		Select("partner_id, COALESCE(AVG(score), 0) AS average, COUNT(*) AS total").
		Where("direction = ?", model.RatingUserToPartner).
		Where("partner_id IN ?", partnerIDs).
		Group("partner_id").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("summarize ratings by partner: %w", err)
	}
	for _, row := range rows {
		out[row.PartnerID] = RatingStats{Average: row.Average, Total: row.Total}
	}
	return out, nil
}

// scope builds the shared WHERE clause for a rating filter.
func (r *GormRatingRepository) scope(f RatingFilter) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		db = db.Model(&model.Rating{}).Where("direction = ?", f.Direction)
		if f.UserID != "" {
			db = db.Where("user_id = ?", f.UserID)
		}
		if f.PartnerID != "" {
			db = db.Where("partner_id = ?", f.PartnerID)
		}
		return db
	}
}
