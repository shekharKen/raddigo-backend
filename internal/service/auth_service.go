package service

import (
	"context"
	"errors"

	"golang.org/x/crypto/bcrypt"

	"github.com/raddigo/raddigo/internal/auth"
	"github.com/raddigo/raddigo/internal/dto"
	"github.com/raddigo/raddigo/internal/repository"
	"github.com/raddigo/raddigo/internal/utils"
)

// AuthService issues and refreshes JWT tokens for users and partners.
type AuthService struct {
	users    repository.UserRepository
	partners repository.PartnerRepository
	tokens   *auth.TokenService
}

// NewAuthService creates an AuthService.
func NewAuthService(users repository.UserRepository, partners repository.PartnerRepository, tokens *auth.TokenService) *AuthService {
	return &AuthService{users: users, partners: partners, tokens: tokens}
}

// LoginUser authenticates a user by email/password and issues a token pair.
func (s *AuthService) LoginUser(ctx context.Context, in dto.LoginRequest) (dto.AuthResponse, error) {
	user, err := s.users.GetByEmail(ctx, in.Email)
	if err != nil {
		if errors.Is(err, utils.ErrNotFound) {
			return dto.AuthResponse{}, utils.ErrInvalidCredentials
		}
		return dto.AuthResponse{}, err
	}

	if err := verifyPassword(user.Password, in.Password); err != nil {
		return dto.AuthResponse{}, err
	}

	if err := isVerified(user.EmailVerified); err != nil {
		return dto.AuthResponse{}, err
	}
	
	pair, err := s.IssueForUser(user.ID)
	if err != nil {
		return dto.AuthResponse{}, err
	}
	
	pair.Info = user
	return pair, nil	
}

// LoginPartner authenticates a partner by email/password and issues a token pair.
func (s *AuthService) LoginPartner(ctx context.Context, in dto.LoginRequest) (dto.AuthResponse, error) {
	partner, err := s.partners.GetByEmail(ctx, in.Email)
	if err != nil {
		if errors.Is(err, utils.ErrNotFound) {
			return dto.AuthResponse{}, utils.ErrInvalidCredentials
		}
		return dto.AuthResponse{}, err
	}

	if err := verifyPassword(partner.Password, in.Password); err != nil {
		return dto.AuthResponse{}, err
	}

	if err := isVerified(partner.EmailVerified); err != nil {
		return dto.AuthResponse{}, err
	}

	pair, err := s.IssueForPartner(partner.ID)
	if err != nil {
		return dto.AuthResponse{}, err
	}
	
	pair.Info = partner
	return pair, nil
}

// Refresh validates a refresh token and issues a new token pair for the same
// subject and role.
func (s *AuthService) Refresh(refreshToken string) (dto.AuthResponse, error) {
	claims, err := s.tokens.ParseRefresh(refreshToken)
	if err != nil {
		return dto.AuthResponse{}, utils.ErrInvalidToken
	}
	return s.issue(claims.Subject, claims.Role)
}

// IssueForUser mints a token pair for a user id, used right after registration.
func (s *AuthService) IssueForUser(userID string) (dto.AuthResponse, error) {
	return s.issue(userID, auth.RoleUser)
}

// IssueForPartner mints a token pair for a partner id, used right after registration.
func (s *AuthService) IssueForPartner(partnerID string) (dto.AuthResponse, error) {
	return s.issue(partnerID, auth.RolePartner)
}

func (s *AuthService) issue(subject string, role auth.Role) (dto.AuthResponse, error) {
	pair, err := s.tokens.Generate(subject, role)
	if err != nil {
		return dto.AuthResponse{}, err
	}
	return dto.AuthResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    pair.ExpiresIn,
	}, nil
}

func verifyPassword(hash, plain string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)); err != nil {
		return utils.ErrInvalidCredentials
	}
	return nil
}

func isVerified(status bool) error {
	if !status {
		return utils.ErrNotVerified
	}
	return nil
}
