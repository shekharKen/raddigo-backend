package validation

import (
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"unicode"

	"github.com/raddigo/raddigo/internal/dto"
	"github.com/raddigo/raddigo/internal/utils"
)

// Field validation limits.
const (
	minPasswordLength = 8
	maxPasswordLength = 12
	maxNameLength     = 50
)

// Validation patterns.
var (
	mobileExtensionRe = regexp.MustCompile(`^\+\d{1,4}$`)
	mobileNoRe        = regexp.MustCompile(`^\d{10}$`)
	nameRe            = regexp.MustCompile(`^[a-zA-Z][a-zA-Z\s'-]*$`)
	timeOfDayRe       = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)
)

// ValidateRegister validates a registration request, returning a
// utils.ValidationError (which matches utils.ErrValidation) describing the
// first field that fails.
func ValidateRegister(in dto.RegisterRequest) error {
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
	for i, a := range in.Addresses {
		if err := validateAddress(i, a); err != nil {
			return err
		}
	}
	return nil
}

func isValidName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > maxNameLength {
		return false
	}
	return nameRe.MatchString(name)
}

func isValidEmail(email string) bool {
	_, err := mail.ParseAddress(strings.TrimSpace(email))
	return err == nil
}

func isValidPassword(password string) bool {
	if len(password) < minPasswordLength || len(password) > maxPasswordLength {
		return false
	}
	var hasLower, hasUpper, hasDigit, hasSpecial bool
	for _, r := range password {
		switch {
		case unicode.IsSpace(r):
			return false // no spaces allowed
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}
	return hasLower && hasUpper && hasDigit && hasSpecial
}

// ValidateUpdateUserProfile validates the provided fields of a user profile.
// All fields are optional; only non-empty fields are validated.
func ValidateUpdateUserProfile(in dto.UpdateUserProfileRequest) error {
	if s := strings.TrimSpace(in.FirstName); s != "" && !isValidName(s) {
		return utils.NewValidationError("first name is invalid: use letters only, up to 50 characters")
	}
	if s := strings.TrimSpace(in.LastName); s != "" && !isValidName(s) {
		return utils.NewValidationError("last name is invalid: use letters only, up to 50 characters")
	}
	if s := strings.TrimSpace(in.MobileExtension); s != "" && !mobileExtensionRe.MatchString(s) {
		return utils.NewValidationError("mobile extension is invalid: expected a '+' followed by 1-4 digits (e.g. +91)")
	}
	if s := strings.TrimSpace(in.MobileNo); s != "" && !mobileNoRe.MatchString(s) {
		return utils.NewValidationError("mobile number is invalid: expected exactly 10 digits")
	}
	return nil
}

// ValidateUpdatePartnerProfile validates the provided fields of a partner
// profile. All fields are optional; only non-empty fields are validated. The
// working-hours range check is done by the service against merged values.
func ValidateUpdatePartnerProfile(in dto.UpdatePartnerProfileRequest) error {
	if s := strings.TrimSpace(in.FirstName); s != "" && !isValidName(s) {
		return utils.NewValidationError("first name is invalid: use letters only, up to 50 characters")
	}
	if s := strings.TrimSpace(in.LastName); s != "" && !isValidName(s) {
		return utils.NewValidationError("last name is invalid: use letters only, up to 50 characters")
	}
	if s := strings.TrimSpace(in.MobileExtension); s != "" && !mobileExtensionRe.MatchString(s) {
		return utils.NewValidationError("mobile extension is invalid: expected a '+' followed by 1-4 digits (e.g. +91)")
	}
	if s := strings.TrimSpace(in.MobileNo); s != "" && !mobileNoRe.MatchString(s) {
		return utils.NewValidationError("mobile number is invalid: expected exactly 10 digits")
	}
	if s := strings.TrimSpace(in.StoreName); s != "" && len(s) > maxStoreLength {
		return utils.NewValidationError(fmt.Sprintf("store name is invalid: up to %d characters", maxStoreLength))
	}
	if s := strings.TrimSpace(in.StartTime); s != "" && !timeOfDayRe.MatchString(s) {
		return utils.NewValidationError("start time is invalid: expected 24-hour format HH:MM (e.g. 09:00)")
	}
	if s := strings.TrimSpace(in.EndTime); s != "" && !timeOfDayRe.MatchString(s) {
		return utils.NewValidationError("end time is invalid: expected 24-hour format HH:MM (e.g. 18:00)")
	}
	return nil
}

// ValidateAddress validates a standalone address payload used by the address
// CRUD endpoints, returning a utils.ValidationError on the first failing field.
func ValidateAddress(a dto.AddressRequest) error {
	if strings.TrimSpace(a.Address1) == "" {
		return utils.NewValidationError("address1 is required")
	}
	if strings.TrimSpace(a.City) == "" {
		return utils.NewValidationError("city is required")
	}
	if strings.TrimSpace(a.Pincode) == "" {
		return utils.NewValidationError("pincode is required")
	}
	if (a.Latitude == nil) != (a.Longitude == nil) {
		return utils.NewValidationError("latitude and longitude must be provided together")
	}
	if a.Latitude != nil && (*a.Latitude < -90 || *a.Latitude > 90) {
		return utils.NewValidationError("latitude must be between -90 and 90")
	}
	if a.Longitude != nil && (*a.Longitude < -180 || *a.Longitude > 180) {
		return utils.NewValidationError("longitude must be between -180 and 180")
	}
	return nil
}

func validateAddress(index int, a dto.AddressRequest) error {
	// Latitude and longitude are optional but must be supplied together and
	// within valid geographic ranges.
	if (a.Latitude == nil) != (a.Longitude == nil) {
		return utils.NewValidationError(fmt.Sprintf("address[%d]: latitude and longitude must be provided together", index))
	}
	if a.Latitude != nil && (*a.Latitude < -90 || *a.Latitude > 90) {
		return utils.NewValidationError(fmt.Sprintf("address[%d]: latitude must be between -90 and 90", index))
	}
	if a.Longitude != nil && (*a.Longitude < -180 || *a.Longitude > 180) {
		return utils.NewValidationError(fmt.Sprintf("address[%d]: longitude must be between -180 and 180", index))
	}
	return nil
}
