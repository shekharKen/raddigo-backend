package dto

// AddressResponse is the API representation of a stored address.
type AddressResponse struct {
	ID        string   `json:"id"`
	Type      string   `json:"type"`
	Address1  string   `json:"address1"`
	Address2  string   `json:"address2"`
	Street    string   `json:"street"`
	City      string   `json:"city"`
	State     string   `json:"state"`
	Country   string   `json:"country"`
	Pincode   string   `json:"pincode"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}
