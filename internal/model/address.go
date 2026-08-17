package model

import "time"

// AddressType identifies which kind of owner an address belongs to.
type AddressType string

const (
	AddressTypeUser         AddressType = "user"
	AddressTypePartnerStore AddressType = "partner_store"
)

// Address is a location owned by either a user or a partner store. A single
// table backs both: Type discriminates the owner and only the matching owner
// id (UserID or PartnerID) is set. City/State/Country/Pincode are indexed for
// fast filtered retrieval.
type Address struct {
	ID        string      `json:"id" gorm:"type:uuid;primaryKey"`
	Type      AddressType `json:"type" gorm:"type:varchar(20);not null;default:user;index"`
	UserID    *string     `json:"user_id,omitempty" gorm:"type:uuid;index"`
	PartnerID *string     `json:"partner_id,omitempty" gorm:"type:uuid;index"`
	Address1  string      `json:"address1"`
	Address2  string      `json:"address2"`
	Street    string      `json:"street"`
	City      string      `json:"city" gorm:"index"`
	State     string      `json:"state" gorm:"index"`
	Country   string      `json:"country" gorm:"index"`
	Pincode   string      `json:"pincode" gorm:"index"`
	Latitude  *float64    `json:"latitude" gorm:"column:latitude"`
	Longitude *float64    `json:"longitude" gorm:"column:longitude"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}
