package validation

import (
	"fmt"
	"strings"

	"github.com/raddigo/raddigo/internal/dto"
	"github.com/raddigo/raddigo/internal/utils"
)

// Rating limits.
const (
	minRatingScore    = 1
	maxRatingScore    = 5
	maxFeedbackLength = 1000
)

// ValidateCreateRating validates a rating request, returning a
// utils.ValidationError describing the first field that fails.
func ValidateCreateRating(in dto.CreateRatingRequest) error {
	if in.Score < minRatingScore || in.Score > maxRatingScore {
		return utils.NewValidationError(fmt.Sprintf("score is invalid: must be between %d and %d", minRatingScore, maxRatingScore))
	}
	if len(strings.TrimSpace(in.Feedback)) > maxFeedbackLength {
		return utils.NewValidationError(fmt.Sprintf("feedback is too long: up to %d characters", maxFeedbackLength))
	}
	return nil
}
