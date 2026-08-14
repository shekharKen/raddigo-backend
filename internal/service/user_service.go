package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
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

// UserService contains user registration and verification logic.
type UserService struct {
	repo    repository.UserRepository
	mailer  mailer.Mailer
	baseURL string
	now     func() time.Time
	id      func() string
	token   func() (string, error)
}

// NewUserService creates a UserService.
func NewUserService(repo repository.UserRepository, m mailer.Mailer, baseURL string) *UserService {
	return &UserService{
		repo:    repo,
		mailer:  m,
		baseURL: strings.TrimRight(baseURL, "/"),
		now:     time.Now,
		id:      func() string { return uuid.NewString() },
		token:   randomToken,
	}
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

	token, err := s.token()
	if err != nil {
		return model.User{}, fmt.Errorf("generate token: %w", err)
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
			UserID:    userID,
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
		VerifyToken:     token,
		Addresses:       addresses,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := s.repo.Create(ctx, &user); err != nil {
		return model.User{}, err
	}

	verifyURL := fmt.Sprintf("%s/api/v1/auth/verify?token=%s", s.baseURL, url.QueryEscape(token))
	if err := s.mailer.SendVerificationEmail(ctx, user.Email, verifyURL); err != nil {
		return model.User{}, fmt.Errorf("send verification email: %w", err)
	}

	return user, nil
}

// VerifyEmail marks the user owning the token as verified.
func (s *UserService) VerifyEmail(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return utils.ErrInvalidToken
	}

	user, err := s.repo.GetByVerifyToken(ctx, token)
	if err != nil {
		if errors.Is(err, utils.ErrNotFound) {
			return utils.ErrInvalidToken
		}
		return err
	}

	return s.repo.MarkEmailVerified(ctx, user.ID)
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
