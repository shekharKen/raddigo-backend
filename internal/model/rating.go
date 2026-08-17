package model

import "time"

// RatingDirection identifies who rated whom for a user/partner pair.
type RatingDirection string

const (
	RatingUserToPartner RatingDirection = "user_to_partner"
	RatingPartnerToUser RatingDirection = "partner_to_user"
)

// Rating is a 1-5 star score with optional feedback exchanged between a user
// and a partner. Direction records which side authored it. The composite
// unique index allows each side to rate the other exactly once (updatable).
type Rating struct {
	ID        string          `json:"id" gorm:"type:uuid;primaryKey"`
	Direction RatingDirection `json:"direction" gorm:"type:varchar(20);not null;uniqueIndex:idx_ratings_pair,priority:3"`
	UserID    string          `json:"user_id" gorm:"type:uuid;not null;uniqueIndex:idx_ratings_pair,priority:1"`
	PartnerID string          `json:"partner_id" gorm:"type:uuid;not null;uniqueIndex:idx_ratings_pair,priority:2"`
	Score     int             `json:"score" gorm:"not null"`
	Feedback  string          `json:"feedback"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}
