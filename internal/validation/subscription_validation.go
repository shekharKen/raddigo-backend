package validation

import (
	"strings"

	"github.com/raddigo/raddigo/internal/dto"
	"github.com/raddigo/raddigo/internal/model"
	"github.com/raddigo/raddigo/internal/utils"
)

// maxPromoCodeLength is the accepted length of a promo code.
const maxPromoCodeLength = 30

// ValidateCreateSubscription validates a new subscription request.
func ValidateCreateSubscription(in dto.CreateSubscriptionRequest) error {
	plan := strings.TrimSpace(in.Plan)
	if plan != string(model.PlanMonthly) && plan != string(model.PlanAnnual) {
		return utils.NewValidationError("plan is invalid: must be 'monthly' or 'annual'")
	}
	if len(strings.TrimSpace(in.PromoCode)) > maxPromoCodeLength {
		return utils.NewValidationError("promo_code is too long")
	}
	return nil
}

// ValidateUpgradeSubscription validates an upgrade request.
func ValidateUpgradeSubscription(in dto.UpgradeSubscriptionRequest) error {
	if len(strings.TrimSpace(in.PromoCode)) > maxPromoCodeLength {
		return utils.NewValidationError("promo_code is too long")
	}
	return nil
}
