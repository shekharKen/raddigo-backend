package model

import "time"

// NotificationRecipientRole identifies which side of a booking a notification
// was sent to.
type NotificationRecipientRole string

const (
	NotificationRecipientUser    NotificationRecipientRole = "user"
	NotificationRecipientPartner NotificationRecipientRole = "partner"
)

// NotificationType identifies the event that triggered a notification.
type NotificationType string

const (
	NotificationBookingRequested NotificationType = "booking_requested"
	NotificationBookingAccepted  NotificationType = "booking_accepted"
	NotificationBookingRejected  NotificationType = "booking_rejected"
	NotificationBookingCancelled NotificationType = "booking_cancelled"
	NotificationBookingCompleted NotificationType = "booking_completed"
	NotificationPickupReminder   NotificationType = "pickup_reminder"
)

// Notification is a single in-app notification record, sent as a push (when
// the recipient has a registered device token) and always kept for the
// recipient's notification inbox.
type Notification struct {
	ID            string                    `json:"id" gorm:"type:uuid;primaryKey"`
	RecipientID   string                    `json:"recipient_id" gorm:"type:uuid;not null;index"`
	RecipientRole NotificationRecipientRole `json:"recipient_role" gorm:"type:varchar(10);not null;index"`
	Type          NotificationType          `json:"type" gorm:"type:varchar(30);not null"`
	Title         string                    `json:"title" gorm:"not null"`
	Body          string                    `json:"body" gorm:"not null"`
	BookingID     *string                   `json:"booking_id,omitempty" gorm:"type:uuid;index"`
	ReadAt        *time.Time                `json:"read_at,omitempty"`
	CreatedAt     time.Time                 `json:"created_at"`
}
