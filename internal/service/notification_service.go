package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/raddigo/raddigo/internal/dto"
	"github.com/raddigo/raddigo/internal/model"
	"github.com/raddigo/raddigo/internal/notification"
	"github.com/raddigo/raddigo/internal/repository"
)

// pushSendTimeout bounds the best-effort async FCM send so it can't leak
// past the goroutine that started it (e.g. after a slow/broken network call).
const pushSendTimeout = 10 * time.Second

// NotificationService sends booking-lifecycle push notifications and keeps an
// in-app notification history for each recipient.
type NotificationService struct {
	notifier notification.Notifier
	repo     repository.NotificationRepository
	logger   *slog.Logger
	now      func() time.Time
	id       func() string
}

// NewNotificationService creates a NotificationService.
func NewNotificationService(notifier notification.Notifier, repo repository.NotificationRepository, logger *slog.Logger) *NotificationService {
	return &NotificationService{
		notifier: notifier,
		repo:     repo,
		logger:   logger,
		now:      time.Now,
		id:       func() string { return uuid.NewString() },
	}
}

// BookingRequested notifies the partner that a user has requested a booking.
func (s *NotificationService) BookingRequested(ctx context.Context, booking model.Booking) {
	if booking.Partner == nil {
		return
	}
	s.notify(ctx, booking.PartnerID, model.NotificationRecipientPartner, booking.Partner.FCMToken,
		model.NotificationBookingRequested, "New booking request",
		"You have a new pickup request. Review and respond.", booking.ID)
}

// BookingAccepted notifies the user that their booking was accepted.
func (s *NotificationService) BookingAccepted(ctx context.Context, booking model.Booking) {
	if booking.User == nil {
		return
	}
	s.notify(ctx, booking.UserID, model.NotificationRecipientUser, booking.User.FCMToken,
		model.NotificationBookingAccepted, "Booking accepted",
		"Your pickup request has been accepted.", booking.ID)
}

// BookingRejected notifies the user that their booking was rejected.
func (s *NotificationService) BookingRejected(ctx context.Context, booking model.Booking) {
	if booking.User == nil {
		return
	}
	s.notify(ctx, booking.UserID, model.NotificationRecipientUser, booking.User.FCMToken,
		model.NotificationBookingRejected, "Booking rejected",
		"Your pickup request has been rejected.", booking.ID)
}

// BookingCancelled notifies the partner that the user cancelled the booking.
func (s *NotificationService) BookingCancelled(ctx context.Context, booking model.Booking) {
	if booking.Partner == nil {
		return
	}
	s.notify(ctx, booking.PartnerID, model.NotificationRecipientPartner, booking.Partner.FCMToken,
		model.NotificationBookingCancelled, "Booking cancelled",
		"The customer cancelled their pickup request.", booking.ID)
}

// BookingCompleted notifies the user that their booking was completed.
func (s *NotificationService) BookingCompleted(ctx context.Context, booking model.Booking) {
	if booking.User == nil {
		return
	}
	s.notify(ctx, booking.UserID, model.NotificationRecipientUser, booking.User.FCMToken,
		model.NotificationBookingCompleted, "Booking completed",
		"Your pickup has been completed. Thanks for using Raddigo!", booking.ID)
}

// PickupReminder notifies both the user and the partner that a scheduled
// pickup is coming up soon. Callers must ensure it is sent at most once per
// booking (see BookingRepository.ClaimReminder).
func (s *NotificationService) PickupReminder(ctx context.Context, booking model.Booking) {
	body := fmt.Sprintf("Pickup scheduled at %s today is coming up soon.", booking.SlotStartTime)
	if booking.User != nil {
		s.notify(ctx, booking.UserID, model.NotificationRecipientUser, booking.User.FCMToken,
			model.NotificationPickupReminder, "Upcoming pickup", body, booking.ID)
	}
	if booking.Partner != nil {
		s.notify(ctx, booking.PartnerID, model.NotificationRecipientPartner, booking.Partner.FCMToken,
			model.NotificationPickupReminder, "Upcoming pickup", body, booking.ID)
	}
}

// List returns a paginated set of a recipient's notifications, newest first.
func (s *NotificationService) List(ctx context.Context, recipientID string, role model.NotificationRecipientRole, page, pageSize int) (dto.PageResult[dto.NotificationResponse], error) {
	page, pageSize = dto.NormalizePageParams(page, pageSize)
	offset := (page - 1) * pageSize

	notifications, total, err := s.repo.ListByRecipient(ctx, recipientID, role, pageSize, offset)
	if err != nil {
		return dto.PageResult[dto.NotificationResponse]{}, err
	}

	out := make([]dto.NotificationResponse, 0, len(notifications))
	for _, n := range notifications {
		out = append(out, dto.NotificationResponse{
			ID:        n.ID,
			Type:      string(n.Type),
			Title:     n.Title,
			Body:      n.Body,
			BookingID: n.BookingID,
			ReadAt:    n.ReadAt,
			CreatedAt: n.CreatedAt,
		})
	}
	return dto.PageResult[dto.NotificationResponse]{
		Data:       out,
		Pagination: dto.NewPagination(page, pageSize, total),
	}, nil
}

// MarkRead flags a recipient's notification as read.
func (s *NotificationService) MarkRead(ctx context.Context, recipientID, id string) error {
	return s.repo.MarkRead(ctx, id, recipientID)
}

// CountUnread returns how many of a recipient's notifications are unread.
func (s *NotificationService) CountUnread(ctx context.Context, recipientID string, role model.NotificationRecipientRole) (int64, error) {
	return s.repo.CountUnread(ctx, recipientID, role)
}

// notify persists the in-app notification synchronously, then best-effort
// sends the push notification asynchronously so a slow/unreachable FCM call
// never blocks the caller's booking action.
func (s *NotificationService) notify(ctx context.Context, recipientID string, role model.NotificationRecipientRole, token string, notifType model.NotificationType, title, body, bookingID string) {
	n := model.Notification{
		ID:            s.id(),
		RecipientID:   recipientID,
		RecipientRole: role,
		Type:          notifType,
		Title:         title,
		Body:          body,
		BookingID:     &bookingID,
		CreatedAt:     s.now(),
	}
	if err := s.repo.Create(ctx, &n); err != nil {
		s.logger.Error("save notification", "error", err, "type", notifType, "recipient_id", recipientID)
	}

	if token == "" {
		return
	}
	go func() {
		sendCtx, cancel := context.WithTimeout(context.Background(), pushSendTimeout)
		defer cancel()
		if err := s.notifier.SendToToken(sendCtx, token, title, body, map[string]string{
			"type":       string(notifType),
			"booking_id": bookingID,
		}); err != nil {
			s.logger.Error("send push notification", "error", err, "type", notifType, "recipient_id", recipientID)
		}
	}()
}
