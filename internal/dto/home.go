package dto

// HomeUser is the minimal set of user details shown on the customer homepage.
type HomeUser struct {
	ID           string `json:"id"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	ProfileImage string `json:"profile_image"`
}

// UserHomeResponse aggregates everything the customer homepage screen needs
// in a single call: greeting details, the primary address, any in-progress
// booking, lifetime stats and the unread notification count.
type UserHomeResponse struct {
	Greeting            string            `json:"greeting"`
	User                HomeUser          `json:"user"`
	PrimaryAddress      *AddressResponse  `json:"primary_address,omitempty"`
	ActiveBooking       *BookingResponse  `json:"active_booking,omitempty"`
	Stats               UserStatsResponse `json:"stats"`
	UnreadNotifications int64             `json:"unread_notifications"`
}

// HomePartner is the minimal set of partner details shown on the partner homepage.
type HomePartner struct {
	ID           string `json:"id"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	StoreName    string `json:"store_name"`
	ProfileImage string `json:"profile_image"`
}

// PartnerHomeResponse aggregates everything the partner homepage screen needs
// in a single call: greeting details, the next accepted pickup, lifetime
// stats, active subscription and the unread notification count.
type PartnerHomeResponse struct {
	Greeting            string                `json:"greeting"`
	Partner             HomePartner           `json:"partner"`
	NextBooking         *BookingResponse      `json:"next_booking,omitempty"`
	Stats               PartnerStatsResponse  `json:"stats"`
	Subscription        *SubscriptionResponse `json:"subscription,omitempty"`
	UnreadNotifications int64                 `json:"unread_notifications"`
}
