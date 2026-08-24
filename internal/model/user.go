package model

import "time"

// User is an application account with one or more addresses.
type User struct {
	ID               string    `json:"id" gorm:"type:uuid;primaryKey"`
	FirstName        string    `json:"first_name" gorm:"not null"`
	LastName         string    `json:"last_name" gorm:"not null"`
	Email            string    `json:"email" gorm:"uniqueIndex;not null"`
	MobileExtension  string    `json:"mobile_extension" gorm:"not null"`
	MobileNo         string    `json:"mobile_no" gorm:"not null"`
	Password         string    `json:"-" gorm:"not null"`
	ProfileImage     string    `json:"profile_image"`
	EmailVerified    bool      `json:"email_verified" gorm:"not null;default:false"`
	VerifyToken      string    `json:"-" gorm:"index"`
	ResetToken       string    `json:"-" gorm:"index"`
	ResetTokenExpiry time.Time `json:"-"`
	Addresses        []Address `json:"addresses" gorm:"constraint:OnDelete:CASCADE"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
