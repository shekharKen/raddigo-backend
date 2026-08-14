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
