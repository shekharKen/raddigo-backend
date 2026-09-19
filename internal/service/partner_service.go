package service

import (
	"context"
	"errors"
	"fmt"
	"math"
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
	repo         repository.PartnerRepository
	ratings      repository.RatingRepository
	mailer       mailer.Mailer
	slotDuration time.Duration
	baseURL      string
	now          func() time.Time
	id           func() string
	token        func() (string, error)
	otp          func() (string, error)
}

// NewPartnerService creates a PartnerService. devOTP, when non-empty, is used
// as a fixed verification OTP instead of a random one (development only).
// baseURL is prefixed onto stored relative image paths (e.g. profile images)
// when returning them to clients.
func NewPartnerService(repo repository.PartnerRepository, ratings repository.RatingRepository, m mailer.Mailer, slotDuration time.Duration, devOTP, baseURL string) *PartnerService {
	if slotDuration <= 0 {
		slotDuration = 30 * time.Minute
	}
	return &PartnerService{
		repo:         repo,
		ratings:      ratings,
		mailer:       m,
		slotDuration: slotDuration,
		baseURL:      baseURL,
		now:          time.Now,
		id:           func() string { return uuid.NewString() },
		token:        randomToken,
		otp:          newOTPFunc(devOTP),
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

	otp, err := s.otp()
	if err != nil {
		return model.Partner{}, fmt.Errorf("generate otp: %w", err)
	}

	now := s.now()
	partnerID := s.id()

	points := buildPolygonPoints(partnerID, in.Polygon, now, s.id)

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
		VerifyOTP:       otp,
		VerifyOTPExpiry: now.Add(otpTTL),
		ServiceArea:     points,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := s.repo.Create(ctx, &partner); err != nil {
		return model.Partner{}, err
	}

	if err := s.mailer.SendVerificationEmail(ctx, partner.Email, otp); err != nil {
		return model.Partner{}, fmt.Errorf("send verification email: %w", err)
	}

	return partner, nil
}

// VerifyEmail confirms the OTP sent to the partner's email and marks the
// account as verified. Already-verified accounts are treated as a no-op success.
func (s *PartnerService) VerifyEmail(ctx context.Context, in dto.VerifyOTPRequest) error {
	if err := validation.ValidateVerifyOTP(in); err != nil {
		return err
	}

	email := strings.ToLower(strings.TrimSpace(in.Email))
	partner, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, utils.ErrNotFound) {
			return utils.ErrInvalidOTP
		}
		return err
	}
	if partner.EmailVerified {
		return nil
	}
	if partner.VerifyOTP == "" || partner.VerifyOTP != strings.TrimSpace(in.OTP) {
		return utils.ErrInvalidOTP
	}
	if partner.VerifyOTPExpiry.IsZero() || s.now().After(partner.VerifyOTPExpiry) {
		return utils.ErrInvalidOTP
	}

	return s.repo.MarkEmailVerified(ctx, partner.ID)
}

// ForgotPassword issues a password reset OTP for the partner with the given
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

	otp, err := s.otp()
	if err != nil {
		return fmt.Errorf("generate otp: %w", err)
	}
	if err := s.repo.SetResetOTP(ctx, partner.ID, otp, s.now().Add(otpTTL)); err != nil {
		return err
	}

	if err := s.mailer.SendPasswordResetEmail(ctx, partner.Email, otp); err != nil {
		return fmt.Errorf("send password reset email: %w", err)
	}
	return nil
}

// VerifyForgotPasswordOTP confirms the password-reset OTP sent to the
// partner's email and, on success, issues a short-lived reset token to be
// used with ResetPassword to actually set the new password.
func (s *PartnerService) VerifyForgotPasswordOTP(ctx context.Context, in dto.VerifyOTPRequest) (string, error) {
	if err := validation.ValidateVerifyOTP(in); err != nil {
		return "", err
	}

	email := strings.ToLower(strings.TrimSpace(in.Email))
	partner, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, utils.ErrNotFound) {
			return "", utils.ErrInvalidOTP
		}
		return "", err
	}
	if partner.ResetOTP == "" || partner.ResetOTP != strings.TrimSpace(in.OTP) {
		return "", utils.ErrInvalidOTP
	}
	if partner.ResetOTPExpiry.IsZero() || s.now().After(partner.ResetOTPExpiry) {
		return "", utils.ErrInvalidOTP
	}

	token, err := s.token()
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	if err := s.repo.SetResetToken(ctx, partner.ID, token, s.now().Add(resetTokenTTL)); err != nil {
		return "", err
	}
	return token, nil
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

// ChangePassword verifies the authenticated partner's current password and, on
// success, replaces it with the new one.
func (s *PartnerService) ChangePassword(ctx context.Context, id string, in dto.ChangePasswordRequest) error {
	if err := validation.ValidateChangePassword(in); err != nil {
		return err
	}

	partner, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(partner.Password), []byte(in.OldPassword)); err != nil {
		return utils.ErrInvalidCredentials
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return s.repo.UpdatePassword(ctx, id, string(hashed))
}

// GetProfile returns the partner with the given id.
func (s *PartnerService) GetProfile(ctx context.Context, id string) (model.Partner, error) {
	partner, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return model.Partner{}, err
	}
	return s.withResolvedImage(partner), nil
}

// UpdateProfile validates and persists the editable profile fields, updating
// only fields that are provided and differ from the current values, and
// returning the updated partner.
func (s *PartnerService) UpdateProfile(ctx context.Context, id string, in dto.UpdatePartnerProfileRequest) (model.Partner, error) {
	if err := validation.ValidateUpdatePartnerProfile(in); err != nil {
		return model.Partner{}, err
	}
	if in.StoreAddress != nil {
		if err := validation.ValidateAddress(*in.StoreAddress); err != nil {
			return model.Partner{}, err
		}
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

	if len(fields) == 0 && in.StoreAddress == nil && in.Polygon == nil {
		return s.withResolvedImage(current), nil
	}

	if len(fields) > 0 {
		fields["updated_at"] = s.now()
		if err := s.repo.UpdateProfile(ctx, id, fields); err != nil {
			return model.Partner{}, err
		}
	}

	if in.StoreAddress != nil {
		address := s.buildStoreAddress(id, *in.StoreAddress)
		if err := s.repo.UpsertStoreAddress(ctx, id, &address); err != nil {
			return model.Partner{}, err
		}
	}

	if in.Polygon != nil {
		points := buildPolygonPoints(id, in.Polygon, s.now(), s.id)
		if err := s.repo.UpsertServiceArea(ctx, id, points); err != nil {
			return model.Partner{}, err
		}
	}

	updated, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return model.Partner{}, err
	}
	return s.withResolvedImage(updated), nil
}

// buildPolygonPoints converts polygon vertices from the request into ordered,
// ID-assigned model rows for the given partner.
func buildPolygonPoints(partnerID string, in []dto.PolygonPointRequest, now time.Time, genID func() string) []model.PolygonPoint {
	points := make([]model.PolygonPoint, 0, len(in))
	for i, p := range in {
		points = append(points, model.PolygonPoint{
			ID:        genID(),
			PartnerID: partnerID,
			Sequence:  i,
			Latitude:  p.Latitude,
			Longitude: p.Longitude,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}
	return points
}

// buildStoreAddress builds a partner store address model from the request input.
func (s *PartnerService) buildStoreAddress(partnerID string, in dto.AddressRequest) model.Address {
	now := s.now()
	return model.Address{
		ID:        s.id(),
		Type:      model.AddressTypePartnerStore,
		PartnerID: &partnerID,
		Address1:  strings.TrimSpace(in.Address1),
		Address2:  trimPtr(in.Address2),
		Street:    strings.TrimSpace(in.Street),
		City:      strings.TrimSpace(in.City),
		State:     strings.TrimSpace(in.State),
		Country:   strings.TrimSpace(in.Country),
		Pincode:   strings.TrimSpace(in.Pincode),
		Latitude:  in.Latitude,
		Longitude: in.Longitude,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// SetProfileImage persists the profile image path for the partner.
func (s *PartnerService) SetProfileImage(ctx context.Context, id, imageURL string) (model.Partner, error) {
	fields := map[string]any{
		"profile_image": imageURL,
		"updated_at":    s.now(),
	}
	if err := s.repo.UpdateProfile(ctx, id, fields); err != nil {
		return model.Partner{}, err
	}
	partner, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return model.Partner{}, err
	}
	return s.withResolvedImage(partner), nil
}

// withResolvedImage returns p with ProfileImage rewritten to an absolute URL.
func (s *PartnerService) withResolvedImage(p model.Partner) model.Partner {
	p.ProfileImage = resolveImageURL(s.baseURL, p.ProfileImage)
	return p
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
			AvailableSlots:  buildTimeSlots(r.StartTime, r.EndTime, s.slotDuration),
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

// buildTimeSlots splits the partner's working window into consecutive slots of
// the given duration. start and end are 24-hour "HH:MM" values; returns nil
// when either is empty, unparseable, or the window is non-positive.
func buildTimeSlots(start, end string, dur time.Duration) []dto.TimeSlot {
	start = strings.TrimSpace(start)
	end = strings.TrimSpace(end)
	if start == "" || end == "" {
		return nil
	}
	if dur <= 0 {
		dur = 30 * time.Minute
	}

	const layout = "15:04"
	startT, err := time.Parse(layout, start)
	if err != nil {
		return nil
	}
	endT, err := time.Parse(layout, end)
	if err != nil {
		return nil
	}
	if !endT.After(startT) {
		return nil
	}

	slots := make([]dto.TimeSlot, 0)
	for cur := startT; cur.Before(endT); cur = cur.Add(dur) {
		next := cur.Add(dur)
		if next.After(endT) {
			next = endT
		}
		slots = append(slots, dto.TimeSlot{
			StartTime: cur.Format(layout),
			EndTime:   next.Format(layout),
		})
	}
	return slots
}
