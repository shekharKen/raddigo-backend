package service

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/raddigo/raddigo/internal/dto"
	"github.com/raddigo/raddigo/internal/model"
	"github.com/raddigo/raddigo/internal/repository"
	"github.com/raddigo/raddigo/internal/utils"
	"github.com/raddigo/raddigo/internal/validation"
)

// RatingService contains the logic for ratings exchanged between users and
// partners in both directions.
type RatingService struct {
	repo repository.RatingRepository
	now  func() time.Time
	id   func() string
}

// NewRatingService creates a RatingService.
func NewRatingService(repo repository.RatingRepository) *RatingService {
	return &RatingService{
		repo: repo,
		now:  time.Now,
		id:   func() string { return uuid.NewString() },
	}
}

// RatePartner records a user's rating of a partner.
func (s *RatingService) RatePartner(ctx context.Context, userID, partnerID string, in dto.CreateRatingRequest) (dto.RatingResponse, error) {
	return s.rate(ctx, model.RatingUserToPartner, userID, partnerID, in)
}

// RateUser records a partner's rating of a user.
func (s *RatingService) RateUser(ctx context.Context, partnerID, userID string, in dto.CreateRatingRequest) (dto.RatingResponse, error) {
	return s.rate(ctx, model.RatingPartnerToUser, userID, partnerID, in)
}

// rate validates the input, confirms both parties exist, and upserts the
// rating for the given direction.
func (s *RatingService) rate(ctx context.Context, direction model.RatingDirection, userID, partnerID string, in dto.CreateRatingRequest) (dto.RatingResponse, error) {
	if err := validation.ValidateCreateRating(in); err != nil {
		return dto.RatingResponse{}, err
	}
	if err := s.ensureUser(ctx, userID); err != nil {
		return dto.RatingResponse{}, err
	}
	if err := s.ensurePartner(ctx, partnerID); err != nil {
		return dto.RatingResponse{}, err
	}

	now := s.now()
	rating := model.Rating{
		ID:        s.id(),
		Direction: direction,
		UserID:    userID,
		PartnerID: partnerID,
		Score:     in.Score,
		Feedback:  strings.TrimSpace(in.Feedback),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.Upsert(ctx, &rating); err != nil {
		return dto.RatingResponse{}, err
	}
	return toRatingResponse(rating), nil
}

// ListForPartner returns the ratings a partner received from users, each
// enriched with the reviewing user's name.
func (s *RatingService) ListForPartner(ctx context.Context, partnerID string, page, pageSize int) (dto.PageResult[dto.RatingResponse], error) {
	if err := s.ensurePartner(ctx, partnerID); err != nil {
		return dto.PageResult[dto.RatingResponse]{}, err
	}

	page, pageSize = dto.NormalizePageParams(page, pageSize)
	offset := (page - 1) * pageSize

	rows, total, err := s.repo.ListReceivedByPartner(ctx, partnerID, pageSize, offset)
	if err != nil {
		return dto.PageResult[dto.RatingResponse]{}, err
	}

	out := make([]dto.RatingResponse, 0, len(rows))
	for _, row := range rows {
		res := toRatingResponse(row.Rating)
		res.RaterFirstName = row.RaterFirstName
		res.RaterLastName = row.RaterLastName
		out = append(out, res)
	}
	return dto.PageResult[dto.RatingResponse]{
		Data:       out,
		Pagination: dto.NewPagination(page, pageSize, total),
	}, nil
}

// ListForUser returns the ratings a user received from partners.
func (s *RatingService) ListForUser(ctx context.Context, userID string, page, pageSize int) (dto.PageResult[dto.RatingResponse], error) {
	if err := s.ensureUser(ctx, userID); err != nil {
		return dto.PageResult[dto.RatingResponse]{}, err
	}
	return s.list(ctx, repository.RatingFilter{Direction: model.RatingPartnerToUser, UserID: userID}, page, pageSize)
}

// SummaryForPartner returns the average score and count a partner received.
func (s *RatingService) SummaryForPartner(ctx context.Context, partnerID string) (dto.RatingSummary, error) {
	if err := s.ensurePartner(ctx, partnerID); err != nil {
		return dto.RatingSummary{}, err
	}
	return s.summary(ctx, repository.RatingFilter{Direction: model.RatingUserToPartner, PartnerID: partnerID})
}

// SummaryForUser returns the average score and count a user received.
func (s *RatingService) SummaryForUser(ctx context.Context, userID string) (dto.RatingSummary, error) {
	if err := s.ensureUser(ctx, userID); err != nil {
		return dto.RatingSummary{}, err
	}
	return s.summary(ctx, repository.RatingFilter{Direction: model.RatingPartnerToUser, UserID: userID})
}

func (s *RatingService) list(ctx context.Context, f repository.RatingFilter, page, pageSize int) (dto.PageResult[dto.RatingResponse], error) {
	page, pageSize = dto.NormalizePageParams(page, pageSize)
	offset := (page - 1) * pageSize

	ratings, total, err := s.repo.List(ctx, f, pageSize, offset)
	if err != nil {
		return dto.PageResult[dto.RatingResponse]{}, err
	}

	out := make([]dto.RatingResponse, 0, len(ratings))
	for _, r := range ratings {
		out = append(out, toRatingResponse(r))
	}
	return dto.PageResult[dto.RatingResponse]{
		Data:       out,
		Pagination: dto.NewPagination(page, pageSize, total),
	}, nil
}

func (s *RatingService) summary(ctx context.Context, f repository.RatingFilter) (dto.RatingSummary, error) {
	avg, total, err := s.repo.Summary(ctx, f)
	if err != nil {
		return dto.RatingSummary{}, err
	}
	return dto.RatingSummary{
		AverageScore: math.Round(avg*100) / 100,
		TotalRatings: total,
	}, nil
}

func (s *RatingService) ensureUser(ctx context.Context, userID string) error {
	exists, err := s.repo.UserExists(ctx, userID)
	if err != nil {
		return err
	}
	if !exists {
		return utils.ErrNotFound
	}
	return nil
}

func (s *RatingService) ensurePartner(ctx context.Context, partnerID string) error {
	exists, err := s.repo.PartnerExists(ctx, partnerID)
	if err != nil {
		return err
	}
	if !exists {
		return utils.ErrNotFound
	}
	return nil
}

func toRatingResponse(r model.Rating) dto.RatingResponse {
	return dto.RatingResponse{
		ID:        r.ID,
		Direction: string(r.Direction),
		UserID:    r.UserID,
		PartnerID: r.PartnerID,
		Score:     r.Score,
		Feedback:  r.Feedback,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}
