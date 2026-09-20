// Package notification sends push notifications via Firebase Cloud Messaging.
package notification

import (
	"context"
	"log/slog"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

// Notifier sends a push notification to a single device token.
type Notifier interface {
	SendToToken(ctx context.Context, token, title, body string, data map[string]string) error
}

// FCMNotifier sends push notifications through the Firebase Admin SDK.
type FCMNotifier struct {
	client *messaging.Client
}

// NewFCMNotifier builds an FCMNotifier from a service-account credentials file.
func NewFCMNotifier(ctx context.Context, credentialsFile string) (*FCMNotifier, error) {
	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsFile(credentialsFile))
	if err != nil {
		return nil, err
	}
	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, err
	}
	return &FCMNotifier{client: client}, nil
}

// SendToToken sends a single push notification to a device token.
func (n *FCMNotifier) SendToToken(ctx context.Context, token, title, body string, data map[string]string) error {
	_, err := n.client.Send(ctx, &messaging.Message{
		Token: token,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: data,
	})
	return err
}

// LogNotifier is a development Notifier that logs instead of sending pushes.
// Used as a fallback when no Firebase credentials are configured.
type LogNotifier struct {
	logger *slog.Logger
}

// NewLogNotifier creates a LogNotifier.
func NewLogNotifier(logger *slog.Logger) *LogNotifier {
	return &LogNotifier{logger: logger}
}

// SendToToken logs the notification that would have been sent.
func (n *LogNotifier) SendToToken(_ context.Context, token, title, body string, data map[string]string) error {
	n.logger.Info("push notification",
		"token", token,
		"title", title,
		"body", body,
		"data", data,
	)
	return nil
}
