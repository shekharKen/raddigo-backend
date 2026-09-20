package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/raddigo/raddigo/internal/model"
	"github.com/raddigo/raddigo/internal/utils"
)

// NotificationRepository defines persistence operations for in-app notifications.
type NotificationRepository interface {
	Create(ctx context.Context, n *model.Notification) error
	ListByRecipient(ctx context.Context, recipientID string, role model.NotificationRecipientRole, limit, offset int) ([]model.Notification, int64, error)
	CountUnread(ctx context.Context, recipientID string, role model.NotificationRecipientRole) (int64, error)
	MarkRead(ctx context.Context, id, recipientID string) error
}

// GormNotificationRepository is a GORM-backed NotificationRepository.
type GormNotificationRepository struct {
	db *gorm.DB
}

// NewGormNotificationRepository creates a GormNotificationRepository.
func NewGormNotificationRepository(db *gorm.DB) *GormNotificationRepository {
	return &GormNotificationRepository{db: db}
}

// Create stores a new notification.
func (r *GormNotificationRepository) Create(ctx context.Context, n *model.Notification) error {
	if err := r.db.WithContext(ctx).Create(n).Error; err != nil {
		return fmt.Errorf("create notification: %w", err)
	}
	return nil
}

// ListByRecipient returns a paginated set of a recipient's notifications,
// newest first.
func (r *GormNotificationRepository) ListByRecipient(ctx context.Context, recipientID string, role model.NotificationRecipientRole, limit, offset int) ([]model.Notification, int64, error) {
	base := func(db *gorm.DB) *gorm.DB {
		return db.Model(&model.Notification{}).
			Where("recipient_id = ? AND recipient_role = ?", recipientID, role)
	}

	var total int64
	if err := base(r.db.WithContext(ctx)).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count notifications: %w", err)
	}

	var notifications []model.Notification
	if err := base(r.db.WithContext(ctx)).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&notifications).Error; err != nil {
		return nil, 0, fmt.Errorf("list notifications: %w", err)
	}
	return notifications, total, nil
}

// CountUnread returns how many of a recipient's notifications are unread.
func (r *GormNotificationRepository) CountUnread(ctx context.Context, recipientID string, role model.NotificationRecipientRole) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.Notification{}).
		Where("recipient_id = ? AND recipient_role = ? AND read_at IS NULL", recipientID, role).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", err)
	}
	return count, nil
}

// MarkRead flags a recipient's notification as read. It returns
// utils.ErrNotFound when the notification does not belong to the recipient.
func (r *GormNotificationRepository) MarkRead(ctx context.Context, id, recipientID string) error {
	now := time.Now()
	res := r.db.WithContext(ctx).
		Model(&model.Notification{}).
		Where("id = ? AND recipient_id = ?", id, recipientID).
		Update("read_at", &now)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return utils.ErrNotFound
		}
		return fmt.Errorf("mark notification read: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return utils.ErrNotFound
	}
	return nil
}
