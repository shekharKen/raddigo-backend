package model

import "time"

// Partner is a scrap dealer account with a store and an operating-area polygon.
type Partner struct {
	ID              string         `json:"id" gorm:"type:uuid;primaryKey"`
	FirstName       string         `json:"first_name" gorm:"not null"`
	LastName        string         `json:"last_name" gorm:"not null"`
	Email           string         `json:"email" gorm:"uniqueIndex;not null"`
	MobileExtension string         `json:"mobile_extension" gorm:"not null"`
	MobileNo        string         `json:"mobile_no" gorm:"not null"`
	Password        string         `json:"-" gorm:"not null"`
	StoreName       string         `json:"store_name" gorm:"not null"`
	StoreAddress    *Address       `json:"store_address" gorm:"foreignKey:PartnerID;constraint:OnDelete:CASCADE"`
	StartTime       string         `json:"start_time" gorm:"not null;default:''"`
	EndTime         string         `json:"end_time" gorm:"not null;default:''"`
	ProfileImage    string         `json:"profile_image"`
	EmailVerified   bool           `json:"email_verified" gorm:"not null;default:false"`
	VerifyToken     string         `json:"-" gorm:"index"`
	ServiceArea     []PolygonPoint `json:"service_area" gorm:"constraint:OnDelete:CASCADE"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// PolygonPoint is a single ordered vertex of a Partner's operating-area polygon.
type PolygonPoint struct {
	ID        string    `json:"id" gorm:"type:uuid;primaryKey"`
	PartnerID string    `json:"partner_id" gorm:"type:uuid;not null;index"`
	Sequence  int       `json:"sequence" gorm:"not null"`
	Latitude  float64   `json:"latitude" gorm:"not null"`
	Longitude float64   `json:"longitude" gorm:"not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
