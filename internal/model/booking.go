package model

import "time"

// BookingStatus is the lifecycle state of a slot booking.
type BookingStatus string

const (
	BookingPending   BookingStatus = "pending"
	BookingAccepted  BookingStatus = "accepted"
	BookingRejected  BookingStatus = "rejected"
	BookingCancelled BookingStatus = "cancelled"
)

// Booking is a user's request to book a partner's time slot for a scrap pickup.
// A partner can accept or reject a pending booking; accepting one marks the slot
// as taken (enforced by a partial unique index on accepted bookings) so no other
// booking for the same partner/date/start-time can be accepted.
type Booking struct {
	ID              string        `json:"id" gorm:"type:uuid;primaryKey"`
	UserID          string        `json:"user_id" gorm:"type:uuid;not null;index"`
	PartnerID       string        `json:"partner_id" gorm:"type:uuid;not null;index"`
	SlotDate        string        `json:"slot_date" gorm:"type:varchar(10);not null"`
	SlotStartTime   string        `json:"slot_start_time" gorm:"type:varchar(5);not null"`
	SlotEndTime     string        `json:"slot_end_time" gorm:"type:varchar(5);not null"`
	Status          BookingStatus `json:"status" gorm:"type:varchar(20);not null;default:pending;index"`
	PickupLatitude  float64       `json:"pickup_latitude" gorm:"not null"`
	PickupLongitude float64       `json:"pickup_longitude" gorm:"not null"`
	PickupAddress   string        `json:"pickup_address"`
	ScrapImage      string        `json:"scrap_image" gorm:"not null"`
	Description     string        `json:"description"`
	Note            string        `json:"note"`
	User            *User         `json:"user,omitempty" gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	Partner         *Partner      `json:"partner,omitempty" gorm:"foreignKey:PartnerID;constraint:OnDelete:CASCADE"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}
