package model

import "time"

// User is an application account with one or more addresses.
type User struct {
	ID              string    `json:"id" gorm:"type:uuid;primaryKey"`
	FirstName       string    `json:"first_name" gorm:"not null"`
	LastName        string    `json:"last_name" gorm:"not null"`
	Email           string    `json:"email" gorm:"uniqueIndex;not null"`
	MobileExtension string    `json:"mobile_extension" gorm:"not null"`
	MobileNo        string    `json:"mobile_no" gorm:"not null"`
	Password        string    `json:"-" gorm:"not null"`
	EmailVerified   bool      `json:"email_verified" gorm:"not null;default:false"`
	VerifyToken     string    `json:"-" gorm:"index"`
	Addresses       []Address `json:"addresses" gorm:"constraint:OnDelete:CASCADE"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Address belongs to a User (many addresses per user).
type Address struct {
	ID        string    `json:"id" gorm:"type:uuid;primaryKey"`
	UserID    string    `json:"user_id" gorm:"type:uuid;not null;index"`
	Address1  string    `json:"address1"`
	Address2  string    `json:"address2"`
	Street    string    `json:"street"`
	City      string    `json:"city"`
	State     string    `json:"state"`
	Country   string    `json:"country"`
	Pincode   string    `json:"pincode"`
	Latitude  *float64  `json:"latitude" gorm:"column:latitude"`
	Longitude *float64  `json:"longitude" gorm:"column:longitude"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
