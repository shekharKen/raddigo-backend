package dto

// RegisterRequest is the payload accepted when registering a new user.
type RegisterRequest struct {
	FirstName       string           `json:"first_name"`
	LastName        string           `json:"last_name"`
	Email           string           `json:"email"`
	MobileExtension string           `json:"mobile_extension"`
	MobileNo        string           `json:"mobile_no"`
	Password        string           `json:"password"`
	Addresses       []AddressRequest `json:"addresses"`
}

// AddressRequest is a single address supplied during registration.
type AddressRequest struct {
	Address1  string   `json:"address1"`
	Address2  *string  `json:"address2"`
	Street    string   `json:"street"`
	City      string   `json:"city"`
	State     string   `json:"state"`
	Country   string   `json:"country"`
	Pincode   string   `json:"pincode"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

// LoginRequest is the payload accepted when authenticating with credentials.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RefreshRequest carries a refresh token to exchange for a new token pair.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// AuthResponse is the issued token set returned by login, refresh and register.
type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	Info         any    `json:"info"`
}

// UpdateUserProfileRequest is the payload accepted when a user edits their profile.
type UpdateUserProfileRequest struct {
	FirstName       string `json:"first_name"`
	LastName        string `json:"last_name"`
	MobileExtension string `json:"mobile_extension"`
	MobileNo        string `json:"mobile_no"`
}

// UpdatePartnerProfileRequest is the payload accepted when a partner edits their profile.
type UpdatePartnerProfileRequest struct {
	FirstName       string `json:"first_name"`
	LastName        string `json:"last_name"`
	MobileExtension string `json:"mobile_extension"`
	MobileNo        string `json:"mobile_no"`
	StoreName       string `json:"store_name"`
	StartTime       string `json:"start_time"`
	EndTime         string `json:"end_time"`
}
