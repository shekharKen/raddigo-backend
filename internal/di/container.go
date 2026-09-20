package di

import (
	"context"
	"log/slog"

	"gorm.io/gorm"

	"github.com/raddigo/raddigo/internal/auth"
	"github.com/raddigo/raddigo/internal/config"
	"github.com/raddigo/raddigo/internal/handler"
	"github.com/raddigo/raddigo/internal/mailer"
	"github.com/raddigo/raddigo/internal/notification"
	"github.com/raddigo/raddigo/internal/repository"
	"github.com/raddigo/raddigo/internal/service"
)

// Repositories groups the data-access layer.
type Repositories struct {
	User         repository.UserRepository
	Partner      repository.PartnerRepository
	Address      repository.AddressRepository
	Rating       repository.RatingRepository
	Booking      repository.BookingRepository
	Subscription repository.SubscriptionRepository
	Notification repository.NotificationRepository
}

// Services groups the business-logic layer.
type Services struct {
	User         *service.UserService
	Partner      *service.PartnerService
	Address      *service.AddressService
	Rating       *service.RatingService
	Booking      *service.BookingService
	Auth         *service.AuthService
	Subscription *service.SubscriptionService
	Notification *service.NotificationService
	Home         *service.HomeService
}

// Handlers groups the HTTP layer.
type Handlers struct {
	Health       *handler.HealthHandler
	Auth         *handler.AuthHandler
	Partner      *handler.PartnerHandler
	Address      *handler.AddressHandler
	Rating       *handler.RatingHandler
	Booking      *handler.BookingHandler
	Profile      *handler.ProfileHandler
	Subscription *handler.SubscriptionHandler
	Notification *handler.NotificationHandler
	Home         *handler.HomeHandler
}

// Container holds the fully wired application dependencies.
type Container struct {
	Repositories Repositories
	Services     Services
	Handlers     Handlers
	Tokens       *auth.TokenService
	Scheduler    *service.ReminderScheduler
}

// New builds the dependency graph layer by layer: repositories, then services,
// then handlers.
func New(cfg config.Config, logger *slog.Logger, db *gorm.DB) *Container {
	repos := buildRepositories(db)
	mail := mailer.NewLogMailer(logger)
	notifier := buildNotifier(cfg, logger)
	tokens := auth.NewTokenService(cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	services := buildServices(cfg, repos, mail, tokens, notifier, logger)
	handlers := buildHandlers(cfg, services)
	scheduler := service.NewReminderScheduler(repos.Booking, services.Notification, cfg.ReminderCheckInterval, cfg.ReminderLeadMinutes, logger)

	return &Container{
		Repositories: repos,
		Services:     services,
		Handlers:     handlers,
		Tokens:       tokens,
		Scheduler:    scheduler,
	}
}

// buildNotifier constructs the FCM notifier from the configured service
// account file, falling back to a log-only notifier (e.g. local development)
// when none is configured or it fails to initialize.
func buildNotifier(cfg config.Config, logger *slog.Logger) notification.Notifier {
	if cfg.FirebaseCredentialsFile == "" {
		logger.Warn("no firebase credentials configured, push notifications will only be logged")
		return notification.NewLogNotifier(logger)
	}
	notifier, err := notification.NewFCMNotifier(context.Background(), cfg.FirebaseCredentialsFile)
	if err != nil {
		logger.Error("init firebase notifier, falling back to log notifier", "error", err)
		return notification.NewLogNotifier(logger)
	}
	return notifier
}

func buildRepositories(db *gorm.DB) Repositories {
	return Repositories{
		User:         repository.NewGormUserRepository(db),
		Partner:      repository.NewGormPartnerRepository(db),
		Address:      repository.NewGormAddressRepository(db),
		Rating:       repository.NewGormRatingRepository(db),
		Booking:      repository.NewGormBookingRepository(db),
		Subscription: repository.NewGormSubscriptionRepository(db),
		Notification: repository.NewGormNotificationRepository(db),
	}
}

func buildServices(cfg config.Config, repos Repositories, mail mailer.Mailer, tokens *auth.TokenService, notifier notification.Notifier, logger *slog.Logger) Services {
	notifications := service.NewNotificationService(notifier, repos.Notification, logger)
	user := service.NewUserService(repos.User, mail, cfg.DevOTP, cfg.AppBaseURL)
	partner := service.NewPartnerService(repos.Partner, repos.Rating, mail, cfg.SlotDuration, cfg.DevOTP, cfg.AppBaseURL)
	address := service.NewAddressService(repos.Address)
	booking := service.NewBookingService(repos.Booking, repos.Partner, repos.Rating, cfg.SlotDuration, cfg.AppBaseURL, mail, cfg.DevOTP, notifications)
	subscription := service.NewSubscriptionService(repos.Subscription, cfg.MonthlySubscriptionPrice, cfg.AnnualSubscriptionPrice)
	return Services{
		User:         user,
		Partner:      partner,
		Address:      address,
		Rating:       service.NewRatingService(repos.Rating),
		Booking:      booking,
		Auth:         service.NewAuthService(repos.User, repos.Partner, repos.Subscription, tokens, cfg.AppBaseURL),
		Subscription: subscription,
		Notification: notifications,
		Home:         service.NewHomeService(user, partner, address, booking, subscription, notifications),
	}
}

func buildHandlers(cfg config.Config, services Services) Handlers {
	return Handlers{
		Health:       handler.NewHealthHandler(),
		Auth:         handler.NewAuthHandler(services.User, services.Auth),
		Partner:      handler.NewPartnerHandler(services.Partner, services.Auth),
		Address:      handler.NewAddressHandler(services.Address),
		Rating:       handler.NewRatingHandler(services.Rating),
		Booking:      handler.NewBookingHandler(services.Booking, cfg.UploadDir),
		Profile:      handler.NewProfileHandler(services.User, services.Partner, cfg.UploadDir),
		Subscription: handler.NewSubscriptionHandler(services.Subscription),
		Notification: handler.NewNotificationHandler(services.Notification),
		Home:         handler.NewHomeHandler(services.Home),
	}
}
