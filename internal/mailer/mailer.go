package mailer

import (
	"context"
	"log/slog"
)

// Mailer sends transactional emails.
type Mailer interface {
	SendVerificationEmail(ctx context.Context, to, verifyURL string) error
}

// LogMailer is a development Mailer that logs emails instead of sending them.
type LogMailer struct {
	logger *slog.Logger
}

// NewLogMailer creates a LogMailer.
func NewLogMailer(logger *slog.Logger) *LogMailer {
	return &LogMailer{logger: logger}
}

// SendVerificationEmail logs the verification link for the recipient.
func (m *LogMailer) SendVerificationEmail(_ context.Context, to, verifyURL string) error {
	m.logger.Info("verification email",
		"to", to,
		"verify_url", verifyURL,
	)
	return nil
}
