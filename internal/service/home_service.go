package service

import (
	"context"
	"errors"
	"time"

	"github.com/raddigo/raddigo/internal/dto"
	"github.com/raddigo/raddigo/internal/model"
	"github.com/raddigo/raddigo/internal/utils"
)

// HomeService aggregates data from other services into the single-call
// payloads the customer and partner homepage screens need.
type HomeService struct {
	users         *UserService
	partners      *PartnerService
	addresses     *AddressService
	bookings      *BookingService
	subscriptions *SubscriptionService
	notifications *NotificationService
	now           func() time.Time
}

// NewHomeService creates a HomeService.
func NewHomeService(users *UserService, partners *PartnerService, addresses *AddressService, bookings *BookingService, subscriptions *SubscriptionService, notifications *NotificationService) *HomeService {
	return &HomeService{
		users:         users,
		partners:      partners,
		addresses:     addresses,
		bookings:      bookings,
		subscriptions: subscriptions,
		notifications: notifications,
		now:           time.Now,
	}
}

// ForUser builds the customer homepage payload: greeting, primary address,
// any in-progress booking, lifetime stats and the unread notification count.
func (s *HomeService) ForUser(ctx context.Context, userID string) (dto.UserHomeResponse, error) {
	user, err := s.users.GetProfile(ctx, userID)
	if err != nil {
		return dto.UserHomeResponse{}, err
	}

	var primaryAddress *dto.AddressResponse
	if len(user.Addresses) > 0 {
		addr := toAddressResponse(user.Addresses[0])
		primaryAddress = &addr
	}

	var activeBooking *dto.BookingResponse
	next, err := s.bookings.NextForUser(ctx, userID)
	if err == nil {
		activeBooking = &next
	} else if !errors.Is(err, utils.ErrNotFound) {
		return dto.UserHomeResponse{}, err
	}

	stats, err := s.bookings.StatsForUser(ctx, userID)
	if err != nil {
		return dto.UserHomeResponse{}, err
	}

	unread, err := s.notifications.CountUnread(ctx, userID, model.NotificationRecipientUser)
	if err != nil {
		return dto.UserHomeResponse{}, err
	}

	return dto.UserHomeResponse{
		Greeting: greetingFor(s.now()),
		User: dto.HomeUser{
			ID:           user.ID,
			FirstName:    user.FirstName,
			LastName:     user.LastName,
			ProfileImage: user.ProfileImage,
		},
		PrimaryAddress:      primaryAddress,
		ActiveBooking:       activeBooking,
		Stats:               stats,
		UnreadNotifications: unread,
	}, nil
}

// ForPartner builds the partner homepage payload: greeting, next accepted
// pickup, lifetime stats, active subscription and the unread notification
// count.
func (s *HomeService) ForPartner(ctx context.Context, partnerID string) (dto.PartnerHomeResponse, error) {
	partner, err := s.partners.GetProfile(ctx, partnerID)
	if err != nil {
		return dto.PartnerHomeResponse{}, err
	}

	var nextBooking *dto.BookingResponse
	next, err := s.bookings.NextForPartner(ctx, partnerID)
	if err == nil {
		nextBooking = &next
	} else if !errors.Is(err, utils.ErrNotFound) {
		return dto.PartnerHomeResponse{}, err
	}

	stats, err := s.bookings.StatsForPartner(ctx, partnerID)
	if err != nil {
		return dto.PartnerHomeResponse{}, err
	}

	var subscription *dto.SubscriptionResponse
	sub, err := s.subscriptions.GetActive(ctx, partnerID)
	if err == nil {
		subscription = &sub
	} else if !errors.Is(err, utils.ErrNotFound) {
		return dto.PartnerHomeResponse{}, err
	}

	unread, err := s.notifications.CountUnread(ctx, partnerID, model.NotificationRecipientPartner)
	if err != nil {
		return dto.PartnerHomeResponse{}, err
	}

	return dto.PartnerHomeResponse{
		Greeting: greetingFor(s.now()),
		Partner: dto.HomePartner{
			ID:           partner.ID,
			FirstName:    partner.FirstName,
			LastName:     partner.LastName,
			StoreName:    partner.StoreName,
			ProfileImage: partner.ProfileImage,
		},
		NextBooking:         nextBooking,
		Stats:               stats,
		Subscription:        subscription,
		UnreadNotifications: unread,
	}, nil
}

// greetingFor returns a time-of-day greeting ("Good morning"/"afternoon"/
// "evening") based on t's local hour.
func greetingFor(t time.Time) string {
	switch h := t.Hour(); {
	case h < 12:
		return "Good morning"
	case h < 17:
		return "Good afternoon"
	default:
		return "Good evening"
	}
}
