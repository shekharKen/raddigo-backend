package validation

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/raddigo/raddigo/internal/dto"
	"github.com/raddigo/raddigo/internal/utils"
)

// Booking field limits.
const (
	maxBookingDescriptionLength = 2000
	maxBookingNoteLength        = 1000
	maxPickupAddressLength      = 500
	bookingDateLayout           = "2006-01-02"
	bookingTimeLayout           = "15:04"
)

// ValidateCreateBooking validates a booking request, returning a
// utils.ValidationError describing the first field that fails. Slot alignment
// against the partner's working hours is validated separately in the service.
func ValidateCreateBooking(in dto.CreateBookingRequest) error {
	if _, err := uuid.Parse(strings.TrimSpace(in.PartnerID)); err != nil {
		return utils.NewValidationError("partner_id is required and must be a valid id")
	}

	date, err := time.Parse(bookingDateLayout, strings.TrimSpace(in.SlotDate))
	if err != nil {
		return utils.NewValidationError("slot_date is invalid: expected format YYYY-MM-DD")
	}
	today := time.Now().Truncate(24 * time.Hour)
	if date.Before(today) {
		return utils.NewValidationError("slot_date must not be in the past")
	}

	start, err := time.Parse(bookingTimeLayout, strings.TrimSpace(in.SlotStartTime))
	if err != nil {
		return utils.NewValidationError("slot_start_time is invalid: expected 24-hour HH:MM")
	}
	end, err := time.Parse(bookingTimeLayout, strings.TrimSpace(in.SlotEndTime))
	if err != nil {
		return utils.NewValidationError("slot_end_time is invalid: expected 24-hour HH:MM")
	}
	if !end.After(start) {
		return utils.NewValidationError("slot_end_time must be after slot_start_time")
	}

	if in.PickupLatitude < -90 || in.PickupLatitude > 90 {
		return utils.NewValidationError("pickup_latitude is invalid: must be between -90 and 90")
	}
	if in.PickupLongitude < -180 || in.PickupLongitude > 180 {
		return utils.NewValidationError("pickup_longitude is invalid: must be between -180 and 180")
	}
	if len(strings.TrimSpace(in.PickupAddress)) > maxPickupAddressLength {
		return utils.NewValidationError(fmt.Sprintf("pickup_address is too long: up to %d characters", maxPickupAddressLength))
	}

	if strings.TrimSpace(in.Description) == "" {
		return utils.NewValidationError("description is required")
	}
	if len(strings.TrimSpace(in.Description)) > maxBookingDescriptionLength {
		return utils.NewValidationError(fmt.Sprintf("description is too long: up to %d characters", maxBookingDescriptionLength))
	}
	if len(strings.TrimSpace(in.Note)) > maxBookingNoteLength {
		return utils.NewValidationError(fmt.Sprintf("note is too long: up to %d characters", maxBookingNoteLength))
	}
	return nil
}
