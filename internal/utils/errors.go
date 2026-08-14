package utils

import "errors"

// ErrNotFound is returned when a requested entity does not exist.
// Service-level sentinel errors.
var (
	ErrValidation   = errors.New("validation failed")
	ErrEmailExists  = errors.New("email already registered")
	ErrInvalidToken = errors.New("invalid or expired verification token")
	ErrNotFound     = errors.New("resource not found")
)

// ValidationError carries a human-readable message for a failed validation
// while still matching ErrValidation via errors.Is.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

func (e *ValidationError) Unwrap() error { return ErrValidation }

// NewValidationError builds a ValidationError with the given message.
func NewValidationError(message string) error {
	return &ValidationError{Message: message}
}

// ErrorResponse is the standard error envelope returned by the API.
type ErrorResponse struct {
	Error string `json:"error"`
}
