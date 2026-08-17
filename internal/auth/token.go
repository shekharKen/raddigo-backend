package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Role identifies the kind of principal a token represents.
type Role string

const (
	RoleUser    Role = "user"
	RolePartner Role = "partner"
)

// tokenType distinguishes short-lived access tokens from refresh tokens.
type tokenType string

const (
	accessToken  tokenType = "access"
	refreshToken tokenType = "refresh"
)

// ErrInvalidToken is returned when a token fails signature, expiry, or type checks.
var ErrInvalidToken = errors.New("invalid token")

// Claims are the JWT claims carried by access and refresh tokens.
type Claims struct {
	Role Role      `json:"role"`
	Type tokenType `json:"typ"`
	jwt.RegisteredClaims
}

// TokenPair is a freshly issued access + refresh token set.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64 // access token lifetime in seconds
}

// TokenService signs and verifies JWT access and refresh tokens.
type TokenService struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	now        func() time.Time
}

// NewTokenService creates a TokenService with the given secret and lifetimes.
func NewTokenService(secret string, accessTTL, refreshTTL time.Duration) *TokenService {
	return &TokenService{
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		now:        time.Now,
	}
}

// Generate issues a new access + refresh token pair for the subject and role.
func (s *TokenService) Generate(subject string, role Role) (TokenPair, error) {
	access, err := s.sign(subject, role, accessToken, s.accessTTL)
	if err != nil {
		return TokenPair{}, err
	}
	refresh, err := s.sign(subject, role, refreshToken, s.refreshTTL)
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(s.accessTTL.Seconds()),
	}, nil
}

func (s *TokenService) sign(subject string, role Role, typ tokenType, ttl time.Duration) (string, error) {
	now := s.now()
	claims := Claims{
		Role: role,
		Type: typ,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

// ParseAccess validates an access token and returns its claims.
func (s *TokenService) ParseAccess(tokenString string) (*Claims, error) {
	return s.parse(tokenString, accessToken)
}

// ParseRefresh validates a refresh token and returns its claims.
func (s *TokenService) ParseRefresh(tokenString string) (*Claims, error) {
	return s.parse(tokenString, refreshToken)
}

func (s *TokenService) parse(tokenString string, expected tokenType) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}
	if claims.Type != expected {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
