package dto

import "time"

// CreateBookingRequest is the payload accepted when a user books a partner's
// slot. It is submitted as multipart/form-data so the scrap image can be sent
// alongside the booking details; ScrapImage is populated by the handler after
// the uploaded file is stored.
type CreateBookingRequest struct {
	PartnerID       string  `json:"partner_id"`
	SlotDate        string  `json:"slot_date"`
	SlotStartTime   string  `json:"slot_start_time"`
	SlotEndTime     string  `json:"slot_end_time"`
	PickupLatitude  float64 `json:"pickup_latitude"`
	PickupLongitude float64 `json:"pickup_longitude"`
	PickupAddress   string  `json:"pickup_address"`
	Description     string  `json:"description"`
	Note            string  `json:"note"`
	ScrapImage      string  `json:"-"`
}

// BookingUser is the limited set of user details exposed to a partner on a
// booking request.
type BookingUser struct {
	ID              string `json:"id"`
	FirstName       string `json:"first_name"`
	LastName        string `json:"last_name"`
	MobileExtension string `json:"mobile_extension"`
	MobileNo        string `json:"mobile_no"`
}

// BookingPartner is the limited set of partner details exposed to a user on a
// booking.
type BookingPartner struct {
	ID              string `json:"id"`
	FirstName       string `json:"first_name"`
	LastName        string `json:"last_name"`
	StoreName       string `json:"store_name"`
	MobileExtension string `json:"mobile_extension"`
	MobileNo        string `json:"mobile_no"`
}

// BookingResponse is the API representation of a stored booking.
type BookingResponse struct {
	ID              string          `json:"id"`
	Status          string          `json:"status"`
	SlotDate        string          `json:"slot_date"`
	SlotStartTime   string          `json:"slot_start_time"`
	SlotEndTime     string          `json:"slot_end_time"`
	PickupLatitude  float64         `json:"pickup_latitude"`
	PickupLongitude float64         `json:"pickup_longitude"`
	PickupAddress   string          `json:"pickup_address"`
	ScrapImage      string          `json:"scrap_image"`
	Description     string          `json:"description"`
	Note            string          `json:"note"`
	User            *BookingUser    `json:"user,omitempty"`
	Partner         *BookingPartner `json:"partner,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}
