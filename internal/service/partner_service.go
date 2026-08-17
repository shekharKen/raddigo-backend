package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/raddigo/raddigo/internal/dto"
	"github.com/raddigo/raddigo/internal/mailer"
	"github.com/raddigo/raddigo/internal/model"
	"github.com/raddigo/raddigo/internal/repository"
	"github.com/raddigo/raddigo/internal/utils"
	"github.com/raddigo/raddigo/internal/validation"
)

// PartnerService contains partner registration and verification logic.
type PartnerService struct {
	repo    repository.PartnerRepository
	ratings repository.RatingRepository
	mailer  mailer.Mailer
	baseURL string
	now     func() time.Time
	id      func() string
	token   func() (string, error)
}

// NewPartnerService creates a PartnerService.
func NewPartnerService(repo repository.PartnerRepository, ratings repository.RatingRepository, m mailer.Mailer, baseURL string) *PartnerService {
	return &PartnerService{
		repo:    repo,
		ratings: ratings,
		mailer:  m,
		baseURL: strings.TrimRight(baseURL, "/"),
		now:     time.Now,
		id:      func() string { return uuid.NewString() },
		token:   randomToken,
	}
}

// Register validates the input, persists the partner with its operating-area
// polygon, and sends a verification email.
func (s *PartnerService) Register(ctx context.Context, in dto.RegisterPartnerRequest) (model.Partner, error) {
	if err := validation.ValidateRegisterPartner(in); err != nil {
		return model.Partner{}, err
	}

	email := strings.ToLower(strings.TrimSpace(in.Email))
	exists, err := s.repo.EmailExists(ctx, email)
	if err != nil {
		return model.Partner{}, err
	}
	if exists {
		return model.Partner{}, utils.ErrEmailExists
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return model.Partner{}, fmt.Errorf("hash password: %w", err)
	}

	token, err := s.token()
	if err != nil {
		return model.Partner{}, fmt.Errorf("generate token: %w", err)
	}

	now := s.now()
	partnerID := s.id()

	points := make([]model.PolygonPoint, 0, len(in.Polygon))
	for i, p := range in.Polygon {
		points = append(points, model.PolygonPoint{
			ID:        s.id(),
			PartnerID: partnerID,
			Sequence:  i,
			Latitude:  p.Latitude,
			Longitude: p.Longitude,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}

	storeAddress := &model.Address{
		ID:        s.id(),
		Type:      model.AddressTypePartnerStore,
		PartnerID: &partnerID,
		Address1:  strings.TrimSpace(in.StoreAddress.Address1),
		Address2:  trimPtr(in.StoreAddress.Address2),
		Street:    strings.TrimSpace(in.StoreAddress.Street),
		City:      strings.TrimSpace(in.StoreAddress.City),
		State:     strings.TrimSpace(in.StoreAddress.State),
		Country:   strings.TrimSpace(in.StoreAddress.Country),
		Pincode:   strings.TrimSpace(in.StoreAddress.Pincode),
		Latitude:  in.StoreAddress.Latitude,
		Longitude: in.StoreAddress.Longitude,
		CreatedAt: now,
		UpdatedAt: now,
	}

	partner := model.Partner{
		ID:              partnerID,
		FirstName:       strings.TrimSpace(in.FirstName),
		LastName:        strings.TrimSpace(in.LastName),
		Email:           email,
		MobileExtension: strings.TrimSpace(in.MobileExtension),
		MobileNo:        strings.TrimSpace(in.MobileNo),
		Password:        string(hashed),
		StoreName:       strings.TrimSpace(in.StoreName),
		StoreAddress:    storeAddress,
		EmailVerified:   false,
		VerifyToken:     token,
		ServiceArea:     points,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := s.repo.Create(ctx, &partner); err != nil {
		return model.Partner{}, err
	}

	verifyURL := fmt.Sprintf("%s/api/v1/auth/partner/verify?token=%s", s.baseURL, url.QueryEscape(token))
	if err := s.mailer.SendVerificationEmail(ctx, partner.Email, verifyURL); err != nil {
		return model.Partner{}, fmt.Errorf("send verification email: %w", err)
	}

	return partner, nil
}

// VerifyEmail marks the partner owning the token as verified.
func (s *PartnerService) VerifyEmail(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return utils.ErrInvalidToken
	}

	partner, err := s.repo.GetByVerifyToken(ctx, token)
	if err != nil {
		if errors.Is(err, utils.ErrNotFound) {
			return utils.ErrInvalidToken
		}
		return err
	}

	return s.repo.MarkEmailVerified(ctx, partner.ID)
}

// SearchByLocation returns a paginated set of verified partners whose operating
// area covers the given coordinates.
func (s *PartnerService) SearchByLocation(ctx context.Context, lat, lng float64, page, pageSize int) (dto.PageResult[dto.PartnerSearchResult], error) {
	if lat < -90 || lat > 90 {
		return dto.PageResult[dto.PartnerSearchResult]{}, utils.NewValidationError("latitude must be between -90 and 90")
	}
	if lng < -180 || lng > 180 {
		return dto.PageResult[dto.PartnerSearchResult]{}, utils.NewValidationError("longitude must be between -180 and 180")
	}

	page, pageSize = dto.NormalizePageParams(page, pageSize)
	offset := (page - 1) * pageSize

	partners, total, err := s.repo.SearchByLocation(ctx, lat, lng, pageSize, offset)
	if err != nil {
		return dto.PageResult[dto.PartnerSearchResult]{}, err
	}

	partnerIDs := make([]string, 0, len(partners))
	for _, r := range partners {
		partnerIDs = append(partnerIDs, r.ID)
	}
	stats, err := s.ratings.SummaryByPartnerIDs(ctx, partnerIDs)
	if err != nil {
		return dto.PageResult[dto.PartnerSearchResult]{}, err
	}

	results := make([]dto.PartnerSearchResult, 0, len(partners))
	for _, r := range partners {
		res := dto.PartnerSearchResult{
			ID:              r.ID,
			FirstName:       r.FirstName,
			LastName:        r.LastName,
			Email:           r.Email,
			MobileExtension: r.MobileExtension,
			MobileNo:        r.MobileNo,
			StoreName:       r.StoreName,
		}
		if stat, ok := stats[r.ID]; ok {
			res.AverageRating = math.Round(stat.Average*100) / 100
			res.TotalRatings = stat.Total
		}
		if r.StoreAddress != nil {
			res.StoreAddress = &dto.AddressResponse{
				ID:        r.StoreAddress.ID,
				Type:      string(r.StoreAddress.Type),
				Address1:  r.StoreAddress.Address1,
				Address2:  r.StoreAddress.Address2,
				Street:    r.StoreAddress.Street,
				City:      r.StoreAddress.City,
				State:     r.StoreAddress.State,
				Country:   r.StoreAddress.Country,
				Pincode:   r.StoreAddress.Pincode,
				Latitude:  r.StoreAddress.Latitude,
				Longitude: r.StoreAddress.Longitude,
			}
		}
		results = append(results, res)
	}

	return dto.PageResult[dto.PartnerSearchResult]{
		Data:       results,
		Pagination: dto.NewPagination(page, pageSize, total),
	}, nil
}

// trimPtr trims a nullable string, returning "" when nil.
func trimPtr(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}
