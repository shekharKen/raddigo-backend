package mailer

import (
	"context"
	"log/slog"
)

// Mailer sends transactional emails.
type Mailer interface {
	SendVerificationEmail(ctx context.Context, to, otp string) error
	SendPasswordResetEmail(ctx context.Context, to, otp string) error
}

// LogMailer is a development Mailer that logs emails instead of sending them.
type LogMailer struct {
	logger *slog.Logger
}

// NewLogMailer creates a LogMailer.
func NewLogMailer(logger *slog.Logger) *LogMailer {
	return &LogMailer{logger: logger}
}

// SendVerificationEmail logs the verification OTP for the recipient.
func (m *LogMailer) SendVerificationEmail(_ context.Context, to, otp string) error {
	m.logger.Info("verification email",
		"to", to,
		"otp", otp,
	)
	return nil
}

// SendPasswordResetEmail logs the password reset OTP for the recipient.
func (m *LogMailer) SendPasswordResetEmail(_ context.Context, to, otp string) error {
	m.logger.Info("password reset email",
		"to", to,
		"otp", otp,
	)
	return nil
}
