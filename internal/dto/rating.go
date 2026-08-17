package dto

import "time"

// CreateRatingRequest is the payload accepted when one side rates the other.
type CreateRatingRequest struct {
	Score    int    `json:"score"`
	Feedback string `json:"feedback"`
}

// RatingResponse is the API representation of a stored rating.
type RatingResponse struct {
	ID             string    `json:"id"`
	Direction      string    `json:"direction"`
	UserID         string    `json:"user_id"`
	PartnerID      string    `json:"partner_id"`
	Score          int       `json:"score"`
	Feedback       string    `json:"feedback"`
	RaterFirstName string    `json:"rater_first_name,omitempty"`
	RaterLastName  string    `json:"rater_last_name,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// RatingSummary aggregates the ratings received by a user or partner.
type RatingSummary struct {
	AverageScore float64 `json:"average_score"`
	TotalRatings int64   `json:"total_ratings"`
}
