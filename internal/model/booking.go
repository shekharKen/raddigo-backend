package model

import "time"

// BookingStatus is the lifecycle state of a slot booking.
type BookingStatus string

const (
	BookingPending   BookingStatus = "pending"
	BookingAccepted  BookingStatus = "accepted"
	BookingRejected  BookingStatus = "rejected"
	BookingCancelled BookingStatus = "cancelled"
	BookingCompleted BookingStatus = "completed"
)

// Booking is a user's request to book a partner's time slot for a scrap pickup.
// A partner can accept or reject a pending booking; accepting one marks the slot
// as taken (enforced by a partial unique index on accepted bookings) so no other
// booking for the same partner/date/start-time can be accepted.
type Booking struct {
	ID              string         `json:"id" gorm:"type:uuid;primaryKey"`
	UserID          string         `json:"user_id" gorm:"type:uuid;not null;index"`
	PartnerID       string         `json:"partner_id" gorm:"type:uuid;not null;index"`
	SlotDate        string         `json:"slot_date" gorm:"type:varchar(10);not null"`
	SlotStartTime   string         `json:"slot_start_time" gorm:"type:varchar(5);not null"`
	SlotEndTime     string         `json:"slot_end_time" gorm:"type:varchar(5);not null"`
	Status          BookingStatus  `json:"status" gorm:"type:varchar(20);not null;default:pending;index"`
	PickupLatitude  float64        `json:"pickup_latitude" gorm:"not null"`
	PickupLongitude float64        `json:"pickup_longitude" gorm:"not null"`
	PickupAddress   string         `json:"pickup_address"`
	Images          []BookingImage `json:"images,omitempty" gorm:"foreignKey:BookingID;constraint:OnDelete:CASCADE"`
	Description     string         `json:"description"`
	Note            string         `json:"note"`
	User            *User          `json:"user,omitempty" gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	Partner         *Partner       `json:"partner,omitempty" gorm:"foreignKey:PartnerID;constraint:OnDelete:CASCADE"`

	// Completion OTP: generated and sent to the user via the send-otp endpoint
	// once a booking is accepted, then read back by the partner to confirm
	// they are handing off to the right customer before it can be completed.
	CompletionOTP       string     `json:"-" gorm:"type:varchar(10)"`
	CompletionOTPExpiry time.Time  `json:"-"`
	OTPVerifiedAt       *time.Time `json:"-"`

	// Completion details, filled in by the partner (alongside OTP verification)
	// once the scrap has been collected and weighed.
	Weight           *float64                 `json:"weight,omitempty"`
	WeightUnit       *string                  `json:"weight_unit,omitempty"`
	AmountPaid       *float64                 `json:"amount_paid,omitempty"`
	CompletionImages []BookingCompletionImage `json:"completion_images,omitempty" gorm:"foreignKey:BookingID;constraint:OnDelete:CASCADE"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BookingCompletionImage is a single ordered scrap image the partner attaches
// when completing a booking (e.g. photos of the weighed scrap), distinct from
// the customer-submitted BookingImage taken at booking time.
type BookingCompletionImage struct {
	ID        string    `json:"id" gorm:"type:uuid;primaryKey"`
	BookingID string    `json:"booking_id" gorm:"type:uuid;not null;index"`
	Sequence  int       `json:"sequence" gorm:"not null"`
	URL       string    `json:"url" gorm:"not null"`
	CreatedAt time.Time `json:"created_at"`
}

// BookingImage is a single ordered scrap image attached to a Booking. The
// ordering of Sequence defines display order.
type BookingImage struct {
	ID        string    `json:"id" gorm:"type:uuid;primaryKey"`
	BookingID string    `json:"booking_id" gorm:"type:uuid;not null;index"`
	Sequence  int       `json:"sequence" gorm:"not null"`
	URL       string    `json:"url" gorm:"not null"`
	CreatedAt time.Time `json:"created_at"`
}

// BookingStatusLog records a single status transition of a Booking, forming
// an append-only audit trail (created, accepted/rejected, completed, cancelled).
type BookingStatusLog struct {
	ID        string        `json:"id" gorm:"type:uuid;primaryKey"`
	BookingID string        `json:"booking_id" gorm:"type:uuid;not null;index"`
	Status    BookingStatus `json:"status" gorm:"type:varchar(20);not null"`
	CreatedAt time.Time     `json:"created_at"`
}
