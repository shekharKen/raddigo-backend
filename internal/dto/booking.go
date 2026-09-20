package dto

import "time"

// MaxBookingImages caps the number of scrap images accepted per booking.
const MaxBookingImages = 5

// MaxCompletionImages caps the number of scrap images the partner attaches
// when completing a booking.
const MaxCompletionImages = 5

// CreateBookingRequest is the payload accepted when a user books a partner's
// slot. It is submitted as multipart/form-data so the scrap images can be sent
// alongside the booking details; ScrapImages is populated by the handler after
// the uploaded files are stored.
type CreateBookingRequest struct {
	PartnerID       string   `json:"partner_id"`
	SlotDate        string   `json:"slot_date"`
	SlotStartTime   string   `json:"slot_start_time"`
	SlotEndTime     string   `json:"slot_end_time"`
	PickupLatitude  float64  `json:"pickup_latitude"`
	PickupLongitude float64  `json:"pickup_longitude"`
	PickupAddress   string   `json:"pickup_address"`
	Description     string   `json:"description"`
	Note            string   `json:"note"`
	ScrapImages     []string `json:"-"`
}

// UpdateBookingRequest is the payload accepted when a user updates a pending
// booking. It is submitted as multipart/form-data like CreateBookingRequest;
// ScrapImages is populated by the handler only when new images are uploaded
// (nil means the existing images are kept, replacing them entirely otherwise).
type UpdateBookingRequest struct {
	SlotDate        string   `json:"slot_date"`
	SlotStartTime   string   `json:"slot_start_time"`
	SlotEndTime     string   `json:"slot_end_time"`
	PickupLatitude  float64  `json:"pickup_latitude"`
	PickupLongitude float64  `json:"pickup_longitude"`
	PickupAddress   string   `json:"pickup_address"`
	Description     string   `json:"description"`
	Note            string   `json:"note"`
	ScrapImages     []string `json:"-"`
}

// VerifyBookingOTPRequest carries the OTP a partner reads back from the
// customer to confirm the handoff before completing a booking.
type VerifyBookingOTPRequest struct {
	OTP string `json:"otp"`
}

// RejectBookingRequest is the payload accepted when a partner rejects a
// pending booking. Reason is optional.
type RejectBookingRequest struct {
	Reason string `json:"reason"`
}

// CancelBookingRequest is the payload accepted when a user cancels a pending
// or accepted booking. Reason is optional.
type CancelBookingRequest struct {
	Reason string `json:"reason"`
}

// CompleteBookingRequest is the payload accepted when a partner completes an
// accepted booking. It is submitted as multipart/form-data so the
// completion scrap images can be sent alongside the weight and amount paid;
// Images is populated by the handler after the uploaded files are stored.
type CompleteBookingRequest struct {
	Weight     float64  `json:"weight"`
	WeightUnit string   `json:"weight_unit"`
	AmountPaid float64  `json:"amount_paid"`
	Images     []string `json:"-"`
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
// booking, including their aggregate rating.
type BookingPartner struct {
	ID              string  `json:"id"`
	FirstName       string  `json:"first_name"`
	LastName        string  `json:"last_name"`
	StoreName       string  `json:"store_name"`
	MobileExtension string  `json:"mobile_extension"`
	MobileNo        string  `json:"mobile_no"`
	AverageRating   float64 `json:"average_rating"`
	TotalRatings    int64   `json:"total_ratings"`
}

// BookingResponse is the API representation of a stored booking.
type BookingResponse struct {
	ID              string   `json:"id"`
	Status          string   `json:"status"`
	SlotDate        string   `json:"slot_date"`
	SlotStartTime   string   `json:"slot_start_time"`
	SlotEndTime     string   `json:"slot_end_time"`
	PickupLatitude  float64  `json:"pickup_latitude"`
	PickupLongitude float64  `json:"pickup_longitude"`
	PickupAddress   string   `json:"pickup_address"`
	Images          []string `json:"images"`
	Description     string   `json:"description"`
	Note            string   `json:"note"`
	// Reason is the optional explanation given when the booking was rejected
	// or cancelled.
	Reason  *string         `json:"reason,omitempty"`
	User    *BookingUser    `json:"user,omitempty"`
	Partner *BookingPartner `json:"partner,omitempty"`
	// OTPVerified reports whether the completion OTP has been confirmed by the
	// partner; required before the booking can be completed.
	OTPVerified      bool      `json:"otp_verified"`
	Weight           *float64  `json:"weight,omitempty"`
	WeightUnit       *string   `json:"weight_unit,omitempty"`
	AmountPaid       *float64  `json:"amount_paid,omitempty"`
	CompletionImages []string  `json:"completion_images,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	// StatusLogs is only populated on the single-booking "get" endpoints.
	StatusLogs []BookingStatusLogResponse `json:"status_logs,omitempty"`
	// Ratings is only populated on the single-booking "get" endpoints. It
	// reflects the overall rating relationship between this booking's user
	// and partner (ratings aren't tracked per booking, so the same values
	// would appear on any other booking shared by this pair).
	Ratings *BookingRatings `json:"ratings,omitempty"`
}

// BookingRatings holds the two possible ratings between a booking's user and
// partner.
type BookingRatings struct {
	UserToPartner *RatingResponse `json:"user_to_partner,omitempty"`
	PartnerToUser *RatingResponse `json:"partner_to_user,omitempty"`
}

// UserStatsResponse is a customer's aggregate booking figures.
type UserStatsResponse struct {
	TotalBookings int64   `json:"total_bookings"`
	TotalKgSent   float64 `json:"total_kg_sent"`
}

// PartnerStatsResponse is a partner's aggregate pickup and earnings figures.
type PartnerStatsResponse struct {
	TotalPickups          int64   `json:"total_pickups"`
	TotalScrapCollectedKg float64 `json:"total_scrap_collected_kg"`
	AverageRating         float64 `json:"average_rating"`
	AmountPaid            float64 `json:"amount_paid"`
}

// BookingStatusLogResponse is a single entry in a booking's status history.
type BookingStatusLogResponse struct {
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}
