package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/raddigo/raddigo/internal/utils"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/raddigo/raddigo/internal/dto"
	"github.com/raddigo/raddigo/internal/mailer"
	"github.com/raddigo/raddigo/internal/model"
	"github.com/raddigo/raddigo/internal/repository"
	"github.com/raddigo/raddigo/internal/validation"
)

// resetTokenTTL is how long a password reset token remains valid.
const resetTokenTTL = time.Hour

// otpTTL is how long an email verification OTP remains valid.
const otpTTL = 15 * time.Minute

// otpLength is the number of digits in a verification OTP.
const otpLength = 6

// UserService contains user registration and verification logic.
type UserService struct {
	repo    repository.UserRepository
	mailer  mailer.Mailer
	baseURL string
	now     func() time.Time
	id      func() string
	token   func() (string, error)
	otp     func() (string, error)
}

// NewUserService creates a UserService. devOTP, when non-empty, is used as a
// fixed verification OTP instead of a random one (development only). baseURL
// is prefixed onto stored relative image paths (e.g. profile images) when
// returning them to clients.
func NewUserService(repo repository.UserRepository, m mailer.Mailer, devOTP, baseURL string) *UserService {
	return &UserService{
		repo:    repo,
		mailer:  m,
		baseURL: baseURL,
		now:     time.Now,
		id:      func() string { return uuid.NewString() },
		token:   randomToken,
		otp:     newOTPFunc(devOTP),
	}
}

// resolveImageURL converts a stored image path into an absolute URL using the
// given base URL. Values that are already absolute (legacy data stored before
// this resolution moved to response time) are returned unchanged, so a change
// to the configured base URL doesn't need a data migration to take effect.
func resolveImageURL(baseURL, path string) string {
	if path == "" || strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return baseURL + path
}

// Register validates the input, persists the user with addresses, and sends a
// verification email.
func (s *UserService) Register(ctx context.Context, in dto.RegisterRequest) (model.User, error) {
	if err := validation.ValidateRegister(in); err != nil {
		fmt.Println(err.Error())
		return model.User{}, err
	}

	email := strings.ToLower(strings.TrimSpace(in.Email))
	exists, err := s.repo.EmailExists(ctx, email)
	if err != nil {
		return model.User{}, err
	}
	if exists {
		return model.User{}, utils.ErrEmailExists
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return model.User{}, fmt.Errorf("hash password: %w", err)
	}

	otp, err := s.otp()
	if err != nil {
		return model.User{}, fmt.Errorf("generate otp: %w", err)
	}

	now := s.now()
	userID := s.id()

	addresses := make([]model.Address, 0, len(in.Addresses))
	for _, a := range in.Addresses {
		address2 := ""
		if a.Address2 != nil {
			address2 = strings.TrimSpace(*a.Address2)
		}
		addresses = append(addresses, model.Address{
			ID:        s.id(),
			Type:      model.AddressTypeUser,
			UserID:    &userID,
			Address1:  strings.TrimSpace(a.Address1),
			Address2:  address2,
			Street:    strings.TrimSpace(a.Street),
			City:      strings.TrimSpace(a.City),
			State:     strings.TrimSpace(a.State),
			Country:   strings.TrimSpace(a.Country),
			Pincode:   strings.TrimSpace(a.Pincode),
			Latitude:  a.Latitude,
			Longitude: a.Longitude,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}

	user := model.User{
		ID:              userID,
		FirstName:       strings.TrimSpace(in.FirstName),
		LastName:        strings.TrimSpace(in.LastName),
		Email:           email,
		MobileExtension: strings.TrimSpace(in.MobileExtension),
		MobileNo:        strings.TrimSpace(in.MobileNo),
		Password:        string(hashed),
		EmailVerified:   false,
		VerifyOTP:       otp,
		VerifyOTPExpiry: now.Add(otpTTL),
		FCMToken:        strings.TrimSpace(in.FCMToken),
		Addresses:       addresses,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := s.repo.Create(ctx, &user); err != nil {
		return model.User{}, err
	}

	if err := s.mailer.SendVerificationEmail(ctx, user.Email, otp); err != nil {
		return model.User{}, fmt.Errorf("send verification email: %w", err)
	}

	return user, nil
}

// GetProfile returns the user with the given id.
func (s *UserService) GetProfile(ctx context.Context, id string) (model.User, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return model.User{}, err
	}
	return s.withResolvedImage(user), nil
}

// UpdateProfile validates and persists the editable profile fields, updating
// only fields that are provided and differ from the current values, and
// returning the updated user.
func (s *UserService) UpdateProfile(ctx context.Context, id string, in dto.UpdateUserProfileRequest) (model.User, error) {
	if err := validation.ValidateUpdateUserProfile(in); err != nil {
		return model.User{}, err
	}
	if in.Address != nil {
		if err := validation.ValidateAddress(*in.Address); err != nil {
			return model.User{}, err
		}
	}

	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return model.User{}, err
	}

	fields := map[string]any{}
	setIfChanged(fields, "first_name", strings.TrimSpace(in.FirstName), current.FirstName)
	setIfChanged(fields, "last_name", strings.TrimSpace(in.LastName), current.LastName)
	setIfChanged(fields, "mobile_extension", strings.TrimSpace(in.MobileExtension), current.MobileExtension)
	setIfChanged(fields, "mobile_no", strings.TrimSpace(in.MobileNo), current.MobileNo)

	if len(fields) == 0 && in.Address == nil {
		return s.withResolvedImage(current), nil
	}

	if len(fields) > 0 {
		fields["updated_at"] = s.now()
		if err := s.repo.UpdateProfile(ctx, id, fields); err != nil {
			return model.User{}, err
		}
	}

	if in.Address != nil {
		address := s.buildAddress(id, *in.Address)
		if err := s.repo.UpsertPrimaryAddress(ctx, id, &address); err != nil {
			return model.User{}, err
		}
	}

	updated, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return model.User{}, err
	}
	return s.withResolvedImage(updated), nil
}

// buildAddress builds a user-owned address model from the request input.
func (s *UserService) buildAddress(userID string, in dto.AddressRequest) model.Address {
	now := s.now()
	return model.Address{
		ID:        s.id(),
		Type:      model.AddressTypeUser,
		UserID:    &userID,
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

// SetProfileImage persists the profile image path for the user.
func (s *UserService) SetProfileImage(ctx context.Context, id, imageURL string) (model.User, error) {
	fields := map[string]any{
		"profile_image": imageURL,
		"updated_at":    s.now(),
	}
	if err := s.repo.UpdateProfile(ctx, id, fields); err != nil {
		return model.User{}, err
	}
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return model.User{}, err
	}
	return s.withResolvedImage(user), nil
}

// withResolvedImage returns u with ProfileImage rewritten to an absolute URL.
func (s *UserService) withResolvedImage(u model.User) model.User {
	u.ProfileImage = resolveImageURL(s.baseURL, u.ProfileImage)
	return u
}

// VerifyEmail confirms the OTP sent to the user's email and marks the account
// as verified. Already-verified accounts are treated as a no-op success.
func (s *UserService) VerifyEmail(ctx context.Context, in dto.VerifyOTPRequest) error {
	if err := validation.ValidateVerifyOTP(in); err != nil {
		return err
	}

	email := strings.ToLower(strings.TrimSpace(in.Email))
	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, utils.ErrNotFound) {
			return utils.ErrInvalidOTP
		}
		return err
	}
	if user.EmailVerified {
		return nil
	}
	if user.VerifyOTP == "" || user.VerifyOTP != strings.TrimSpace(in.OTP) {
		return utils.ErrInvalidOTP
	}
	if user.VerifyOTPExpiry.IsZero() || s.now().After(user.VerifyOTPExpiry) {
		return utils.ErrInvalidOTP
	}

	return s.repo.MarkEmailVerified(ctx, user.ID)
}

// ForgotPassword issues a password reset OTP for the account with the given
// email and sends it by email. To avoid leaking which emails are registered, it
// returns nil when no account matches.
func (s *UserService) ForgotPassword(ctx context.Context, in dto.ForgotPasswordRequest) error {
	if err := validation.ValidateForgotPassword(in); err != nil {
		return err
	}

	email := strings.ToLower(strings.TrimSpace(in.Email))
	user, err := s.repo.GetByEmail(ctx, email)
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
	if err := s.repo.SetResetOTP(ctx, user.ID, otp, s.now().Add(otpTTL)); err != nil {
		return err
	}

	if err := s.mailer.SendPasswordResetEmail(ctx, user.Email, otp); err != nil {
		return fmt.Errorf("send password reset email: %w", err)
	}
	return nil
}

// VerifyForgotPasswordOTP confirms the password-reset OTP sent to the user's
// email and, on success, issues a short-lived reset token to be used with
// ResetPassword to actually set the new password.
func (s *UserService) VerifyForgotPasswordOTP(ctx context.Context, in dto.VerifyOTPRequest) (string, error) {
	if err := validation.ValidateVerifyOTP(in); err != nil {
		return "", err
	}

	email := strings.ToLower(strings.TrimSpace(in.Email))
	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, utils.ErrNotFound) {
			return "", utils.ErrInvalidOTP
		}
		return "", err
	}
	if user.ResetOTP == "" || user.ResetOTP != strings.TrimSpace(in.OTP) {
		return "", utils.ErrInvalidOTP
	}
	if user.ResetOTPExpiry.IsZero() || s.now().After(user.ResetOTPExpiry) {
		return "", utils.ErrInvalidOTP
	}

	token, err := s.token()
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	if err := s.repo.SetResetToken(ctx, user.ID, token, s.now().Add(resetTokenTTL)); err != nil {
		return "", err
	}
	return token, nil
}

// ResetPassword validates the reset token and, if valid and unexpired, sets the
// account's new password and clears the token.
func (s *UserService) ResetPassword(ctx context.Context, in dto.ResetPasswordRequest) error {
	if err := validation.ValidateResetPassword(in); err != nil {
		return err
	}

	user, err := s.repo.GetByResetToken(ctx, strings.TrimSpace(in.Token))
	if err != nil {
		if errors.Is(err, utils.ErrNotFound) {
			return utils.ErrInvalidToken
		}
		return err
	}
	if user.ResetTokenExpiry.IsZero() || s.now().After(user.ResetTokenExpiry) {
		return utils.ErrInvalidToken
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return s.repo.UpdatePassword(ctx, user.ID, string(hashed))
}

// ChangePassword verifies the authenticated user's current password and, on
// success, replaces it with the new one.
func (s *UserService) ChangePassword(ctx context.Context, id string, in dto.ChangePasswordRequest) error {
	if err := validation.ValidateChangePassword(in); err != nil {
		return err
	}

	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(in.OldPassword)); err != nil {
		return utils.ErrInvalidCredentials
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return s.repo.UpdatePassword(ctx, id, string(hashed))
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// newOTPFunc returns a generator that always yields devOTP when non-empty
// (development only), or a random OTP otherwise.
func newOTPFunc(devOTP string) func() (string, error) {
	devOTP = strings.TrimSpace(devOTP)
	if devOTP != "" {
		return func() (string, error) { return devOTP, nil }
	}
	return randomOTP
}

// randomOTP generates a random numeric OTP of otpLength digits.
func randomOTP() (string, error) {
	const digits = "0123456789"
	b := make([]byte, otpLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i, v := range b {
		b[i] = digits[int(v)%len(digits)]
	}
	return string(b), nil
}
