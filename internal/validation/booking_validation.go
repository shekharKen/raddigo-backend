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
	maxWeightGrams              = 999
	maxAmountPaid               = 10_000_000
)

// ValidateCreateBooking validates a booking request, returning a
// utils.ValidationError describing the first field that fails. Slot alignment
// against the partner's working hours is validated separately in the service.
func ValidateCreateBooking(in dto.CreateBookingRequest) error {
	if _, err := uuid.Parse(strings.TrimSpace(in.PartnerID)); err != nil {
		return utils.NewValidationError("partner_id is required and must be a valid id")
	}
	if err := validateBookingSlot(in.SlotDate, in.SlotStartTime, in.SlotEndTime); err != nil {
		return err
	}
	if err := validateBookingPickup(in.PickupLatitude, in.PickupLongitude, in.PickupAddress); err != nil {
		return err
	}
	if err := validateBookingImages(in.ScrapImages); err != nil {
		return err
	}
	return validateBookingNotes(in.Description, in.Note)
}

// ValidateUpdateBooking validates a booking update request, using the same
// field rules as ValidateCreateBooking minus the immutable partner_id.
func ValidateUpdateBooking(in dto.UpdateBookingRequest) error {
	if err := validateBookingSlot(in.SlotDate, in.SlotStartTime, in.SlotEndTime); err != nil {
		return err
	}
	if err := validateBookingPickup(in.PickupLatitude, in.PickupLongitude, in.PickupAddress); err != nil {
		return err
	}
	// nil ScrapImages means the existing images are kept unchanged.
	if in.ScrapImages != nil {
		if err := validateBookingImages(in.ScrapImages); err != nil {
			return err
		}
	}
	return validateBookingNotes(in.Description, in.Note)
}

// validateBookingSlot validates the slot date/time fields shared by create
// and update requests.
func validateBookingSlot(slotDate, slotStartTime, slotEndTime string) error {
	date, err := time.Parse(bookingDateLayout, strings.TrimSpace(slotDate))
	if err != nil {
		return utils.NewValidationError("slot_date is invalid: expected format YYYY-MM-DD")
	}
	today := time.Now().Truncate(24 * time.Hour)
	if date.Before(today) {
		return utils.NewValidationError("slot_date must not be in the past")
	}

	start, err := time.Parse(bookingTimeLayout, strings.TrimSpace(slotStartTime))
	if err != nil {
		return utils.NewValidationError("slot_start_time is invalid: expected 24-hour HH:MM")
	}
	end, err := time.Parse(bookingTimeLayout, strings.TrimSpace(slotEndTime))
	if err != nil {
		return utils.NewValidationError("slot_end_time is invalid: expected 24-hour HH:MM")
	}
	if !end.After(start) {
		return utils.NewValidationError("slot_end_time must be after slot_start_time")
	}
	return nil
}

// validateBookingPickup validates the pickup location fields shared by create
// and update requests.
func validateBookingPickup(lat, lng float64, address string) error {
	if lat < -90 || lat > 90 {
		return utils.NewValidationError("pickup_latitude is invalid: must be between -90 and 90")
	}
	if lng < -180 || lng > 180 {
		return utils.NewValidationError("pickup_longitude is invalid: must be between -180 and 180")
	}
	if len(strings.TrimSpace(address)) > maxPickupAddressLength {
		return utils.NewValidationError(fmt.Sprintf("pickup_address is too long: up to %d characters", maxPickupAddressLength))
	}
	return nil
}

// validateBookingImages validates the number of scrap images attached to a
// create or update request. Images are optional for now, up to the cap.
func validateBookingImages(images []string) error {
	if len(images) > dto.MaxBookingImages {
		return utils.NewValidationError(fmt.Sprintf("too many images: up to %d allowed", dto.MaxBookingImages))
	}
	return nil
}

// validateBookingNotes validates the description/note fields shared by create
// and update requests.
func validateBookingNotes(description, note string) error {
	if strings.TrimSpace(description) == "" {
		return utils.NewValidationError("description is required")
	}
	if len(strings.TrimSpace(description)) > maxBookingDescriptionLength {
		return utils.NewValidationError(fmt.Sprintf("description is too long: up to %d characters", maxBookingDescriptionLength))
	}
	if len(strings.TrimSpace(note)) > maxBookingNoteLength {
		return utils.NewValidationError(fmt.Sprintf("note is too long: up to %d characters", maxBookingNoteLength))
	}
	return nil
}

// ValidateVerifyBookingOTP validates a booking completion OTP submission.
func ValidateVerifyBookingOTP(in dto.VerifyBookingOTPRequest) error {
	if !otpRe.MatchString(strings.TrimSpace(in.OTP)) {
		return utils.NewValidationError("otp is invalid: expected a 6-digit code")
	}
	return nil
}

// ValidateCompleteBooking validates the completion details a partner submits
// (scrap images, weight and amount paid) to complete an accepted booking.
func ValidateCompleteBooking(in dto.CompleteBookingRequest) error {
	if len(in.Images) == 0 {
		return utils.NewValidationError("at least one scrap image is required")
	}
	if len(in.Images) > dto.MaxCompletionImages {
		return utils.NewValidationError(fmt.Sprintf("too many images: up to %d allowed", dto.MaxCompletionImages))
	}
	if in.WeightKg < 0 {
		return utils.NewValidationError("weight_kg is invalid: must not be negative")
	}
	if in.WeightGrams < 0 || in.WeightGrams > maxWeightGrams {
		return utils.NewValidationError(fmt.Sprintf("weight_grams is invalid: must be between 0 and %d", maxWeightGrams))
	}
	if in.WeightKg == 0 && in.WeightGrams == 0 {
		return utils.NewValidationError("weight is required: weight_kg and weight_grams must not both be zero")
	}
	if in.AmountPaid < 0 || in.AmountPaid > maxAmountPaid {
		return utils.NewValidationError("amount_paid is invalid: must be a non-negative amount")
	}
	return nil
}
