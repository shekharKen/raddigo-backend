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
		StartTime:       strings.TrimSpace(in.StartTime),
		EndTime:         strings.TrimSpace(in.EndTime),
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

// ForgotPassword issues a password reset token for the partner with the given
// email and sends it by email. To avoid leaking which emails are registered, it
// returns nil when no partner matches.
func (s *PartnerService) ForgotPassword(ctx context.Context, in dto.ForgotPasswordRequest) error {
	if err := validation.ValidateForgotPassword(in); err != nil {
		return err
	}

	email := strings.ToLower(strings.TrimSpace(in.Email))
	partner, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, utils.ErrNotFound) {
			return nil
		}
		return err
	}

	token, err := s.token()
	if err != nil {
		return fmt.Errorf("generate token: %w", err)
	}
	if err := s.repo.SetResetToken(ctx, partner.ID, token, s.now().Add(resetTokenTTL)); err != nil {
		return err
	}

	resetURL := fmt.Sprintf("%s/api/v1/auth/partner/reset-password?token=%s", s.baseURL, url.QueryEscape(token))
	if err := s.mailer.SendPasswordResetEmail(ctx, partner.Email, resetURL); err != nil {
		return fmt.Errorf("send password reset email: %w", err)
	}
	return nil
}

// ResetPassword validates the reset token and, if valid and unexpired, sets the
// partner's new password and clears the token.
func (s *PartnerService) ResetPassword(ctx context.Context, in dto.ResetPasswordRequest) error {
	if err := validation.ValidateResetPassword(in); err != nil {
		return err
	}

	partner, err := s.repo.GetByResetToken(ctx, strings.TrimSpace(in.Token))
	if err != nil {
		if errors.Is(err, utils.ErrNotFound) {
			return utils.ErrInvalidToken
		}
		return err
	}
	if partner.ResetTokenExpiry.IsZero() || s.now().After(partner.ResetTokenExpiry) {
		return utils.ErrInvalidToken
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return s.repo.UpdatePassword(ctx, partner.ID, string(hashed))
}

// GetProfile returns the partner with the given id.
func (s *PartnerService) GetProfile(ctx context.Context, id string) (model.Partner, error) {
	return s.repo.GetByID(ctx, id)
}

// UpdateProfile validates and persists the editable profile fields, updating
// only fields that are provided and differ from the current values, and
// returning the updated partner.
func (s *PartnerService) UpdateProfile(ctx context.Context, id string, in dto.UpdatePartnerProfileRequest) (model.Partner, error) {
	if err := validation.ValidateUpdatePartnerProfile(in); err != nil {
		return model.Partner{}, err
	}

	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return model.Partner{}, err
	}

	fields := map[string]any{}
	setIfChanged(fields, "first_name", strings.TrimSpace(in.FirstName), current.FirstName)
	setIfChanged(fields, "last_name", strings.TrimSpace(in.LastName), current.LastName)
	setIfChanged(fields, "mobile_extension", strings.TrimSpace(in.MobileExtension), current.MobileExtension)
	setIfChanged(fields, "mobile_no", strings.TrimSpace(in.MobileNo), current.MobileNo)
	setIfChanged(fields, "store_name", strings.TrimSpace(in.StoreName), current.StoreName)

	// Validate the working-hours range against the merged (new-or-current) values.
	if strings.TrimSpace(in.StartTime) != "" || strings.TrimSpace(in.EndTime) != "" {
		start := valueOr(strings.TrimSpace(in.StartTime), current.StartTime)
		end := valueOr(strings.TrimSpace(in.EndTime), current.EndTime)
		if start != "" && end != "" {
			if err := validation.ValidateWorkingHours(start, end); err != nil {
				return model.Partner{}, err
			}
		}
		setIfChanged(fields, "start_time", strings.TrimSpace(in.StartTime), current.StartTime)
		setIfChanged(fields, "end_time", strings.TrimSpace(in.EndTime), current.EndTime)
	}

	if len(fields) == 0 {
		return current, nil
	}
	fields["updated_at"] = s.now()
	if err := s.repo.UpdateProfile(ctx, id, fields); err != nil {
		return model.Partner{}, err
	}
	return s.repo.GetByID(ctx, id)
}

// SetProfileImage persists the profile image URL for the partner.
func (s *PartnerService) SetProfileImage(ctx context.Context, id, imageURL string) (model.Partner, error) {
	fields := map[string]any{
		"profile_image": imageURL,
		"updated_at":    s.now(),
	}
	if err := s.repo.UpdateProfile(ctx, id, fields); err != nil {
		return model.Partner{}, err
	}
	return s.repo.GetByID(ctx, id)
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
			AvailableSlots:  buildTimeSlots(r.StartTime, r.EndTime),
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

// setIfChanged adds key=val to fields when val is non-empty and differs from
// the current value, so blank inputs leave existing data untouched.
func setIfChanged(fields map[string]any, key, val, current string) {
	if val != "" && val != current {
		fields[key] = val
	}
}

// valueOr returns val when it is non-empty, otherwise the fallback.
func valueOr(val, fallback string) string {
	if val != "" {
		return val
	}
	return fallback
}

// buildTimeSlots returns the partner's working window as a single slot.
// start and end are 24-hour "HH:MM" values; returns nil when either is empty.
func buildTimeSlots(start, end string) []dto.TimeSlot {
	start = strings.TrimSpace(start)
	end = strings.TrimSpace(end)
	if start == "" || end == "" {
		return nil
	}
	return []dto.TimeSlot{{StartTime: start, EndTime: end}}
}
