package dto

// RegisterPartnerRequest is the payload accepted when registering a new partner.
type RegisterPartnerRequest struct {
	FirstName       string                `json:"first_name"`
	LastName        string                `json:"last_name"`
	Email           string                `json:"email"`
	MobileExtension string                `json:"mobile_extension"`
	MobileNo        string                `json:"mobile_no"`
	Password        string                `json:"password"`
	StoreName       string                `json:"store_name"`
	StoreAddress    AddressRequest        `json:"store_address"`
	Polygon         []PolygonPointRequest `json:"polygon"`
}

// PolygonPointRequest is a single vertex of the partner's operating area.
// The ordering of the slice defines the polygon boundary.
type PolygonPointRequest struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// PartnerSearchResult is a partner returned by a location search.
type PartnerSearchResult struct {
	ID              string           `json:"id"`
	FirstName       string           `json:"first_name"`
	LastName        string           `json:"last_name"`
	Email           string           `json:"email"`
	MobileExtension string           `json:"mobile_extension"`
	MobileNo        string           `json:"mobile_no"`
	StoreName       string           `json:"store_name"`
	StoreAddress    *AddressResponse `json:"store_address,omitempty"`
	AverageRating   float64          `json:"average_rating"`
	TotalRatings    int64            `json:"total_ratings"`
}
