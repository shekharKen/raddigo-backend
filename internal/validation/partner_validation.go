package validation

import (
	"fmt"
	"strings"

	"github.com/raddigo/raddigo/internal/dto"
	"github.com/raddigo/raddigo/internal/utils"
)

// Polygon boundaries.
const (
	minPolygonPoints = 3
	maxStoreLength   = 100
)

// ValidateRegisterPartner validates a partner registration request, returning a
// utils.ValidationError (which matches utils.ErrValidation) describing the
// first field that fails.
func ValidateRegisterPartner(in dto.RegisterPartnerRequest) error {
	if !isValidName(in.FirstName) {
		return utils.NewValidationError("first name is invalid: use letters only, up to 50 characters")
	}
	if !isValidName(in.LastName) {
		return utils.NewValidationError("last name is invalid: use letters only, up to 50 characters")
	}
	if !mobileExtensionRe.MatchString(strings.TrimSpace(in.MobileExtension)) {
		return utils.NewValidationError("mobile extension is invalid: expected a '+' followed by 1-4 digits (e.g. +91)")
	}
	if !mobileNoRe.MatchString(strings.TrimSpace(in.MobileNo)) {
		return utils.NewValidationError("mobile number is invalid: expected exactly 10 digits")
	}
	if !isValidEmail(in.Email) {
		return utils.NewValidationError("email is invalid")
	}
	if !isValidPassword(in.Password) {
		return utils.NewValidationError(fmt.Sprintf("password must be %d-%d characters with no spaces and include at least one lowercase letter, one uppercase letter, one number and one special character", minPasswordLength, maxPasswordLength))
	}
	if name := strings.TrimSpace(in.StoreName); name == "" || len(name) > maxStoreLength {
		return utils.NewValidationError(fmt.Sprintf("store name is invalid: required, up to %d characters", maxStoreLength))
	}
	if err := ValidateAddress(in.StoreAddress); err != nil {
		return err
	}
	if err := ValidateWorkingHours(in.StartTime, in.EndTime); err != nil {
		return err
	}
	if err := validatePolygon(in.Polygon); err != nil {
		return err
	}
	return nil
}

// ValidateWorkingHours checks that the store's opening and closing times are
// valid 24-hour "HH:MM" values and that the closing time is after the opening
// time.
func ValidateWorkingHours(start, end string) error {
	start = strings.TrimSpace(start)
	end = strings.TrimSpace(end)
	if !timeOfDayRe.MatchString(start) {
		return utils.NewValidationError("working start time is invalid: expected 24-hour format HH:MM (e.g. 09:00)")
	}
	if !timeOfDayRe.MatchString(end) {
		return utils.NewValidationError("working end time is invalid: expected 24-hour format HH:MM (e.g. 18:00)")
	}
	if end <= start {
		return utils.NewValidationError("working end time must be after working start time")
	}
	return nil
}

func validatePolygon(points []dto.PolygonPointRequest) error {
	if len(points) < minPolygonPoints {
		return utils.NewValidationError(fmt.Sprintf("polygon is invalid: at least %d coordinates are required", minPolygonPoints))
	}
	for i, p := range points {
		if p.Latitude < -90 || p.Latitude > 90 {
			return utils.NewValidationError(fmt.Sprintf("polygon[%d]: latitude must be between -90 and 90", i))
		}
		if p.Longitude < -180 || p.Longitude > 180 {
			return utils.NewValidationError(fmt.Sprintf("polygon[%d]: longitude must be between -180 and 180", i))
		}
	}
	return nil
}
