package di

import (
	"log/slog"

	"gorm.io/gorm"

	"github.com/raddigo/raddigo/internal/auth"
	"github.com/raddigo/raddigo/internal/config"
	"github.com/raddigo/raddigo/internal/handler"
	"github.com/raddigo/raddigo/internal/mailer"
	"github.com/raddigo/raddigo/internal/repository"
	"github.com/raddigo/raddigo/internal/service"
)

// Repositories groups the data-access layer.
type Repositories struct {
	User    repository.UserRepository
	Partner repository.PartnerRepository
	Address repository.AddressRepository
	Rating  repository.RatingRepository
	Booking repository.BookingRepository
}

// Services groups the business-logic layer.
type Services struct {
	User    *service.UserService
	Partner *service.PartnerService
	Address *service.AddressService
	Rating  *service.RatingService
	Booking *service.BookingService
	Auth    *service.AuthService
}

// Handlers groups the HTTP layer.
type Handlers struct {
	Health  *handler.HealthHandler
	Auth    *handler.AuthHandler
	Partner *handler.PartnerHandler
	Address *handler.AddressHandler
	Rating  *handler.RatingHandler
	Booking *handler.BookingHandler
	Profile *handler.ProfileHandler
}

// Container holds the fully wired application dependencies.
type Container struct {
	Repositories Repositories
	Services     Services
	Handlers     Handlers
	Tokens       *auth.TokenService
}

// New builds the dependency graph layer by layer: repositories, then services,
// then handlers.
func New(cfg config.Config, logger *slog.Logger, db *gorm.DB) *Container {
	repos := buildRepositories(db)
	mail := mailer.NewLogMailer(logger)
	tokens := auth.NewTokenService(cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	services := buildServices(cfg, repos, mail, tokens)
	handlers := buildHandlers(cfg, services)

	return &Container{
		Repositories: repos,
		Services:     services,
		Handlers:     handlers,
		Tokens:       tokens,
	}
}

func buildRepositories(db *gorm.DB) Repositories {
	return Repositories{
		User:    repository.NewGormUserRepository(db),
		Partner: repository.NewGormPartnerRepository(db),
		Address: repository.NewGormAddressRepository(db),
		Rating:  repository.NewGormRatingRepository(db),
		Booking: repository.NewGormBookingRepository(db),
	}
}

func buildServices(cfg config.Config, repos Repositories, mail mailer.Mailer, tokens *auth.TokenService) Services {
	return Services{
		User:    service.NewUserService(repos.User, mail, cfg.AppBaseURL),
		Partner: service.NewPartnerService(repos.Partner, repos.Rating, mail, cfg.AppBaseURL, cfg.SlotDuration),
		Address: service.NewAddressService(repos.Address),
		Rating:  service.NewRatingService(repos.Rating),
		Booking: service.NewBookingService(repos.Booking, repos.Partner, cfg.SlotDuration),
		Auth:    service.NewAuthService(repos.User, repos.Partner, tokens),
	}
}

func buildHandlers(cfg config.Config, services Services) Handlers {
	return Handlers{
		Health:  handler.NewHealthHandler(),
		Auth:    handler.NewAuthHandler(services.User, services.Auth),
		Partner: handler.NewPartnerHandler(services.Partner, services.Auth),
		Address: handler.NewAddressHandler(services.Address),
		Rating:  handler.NewRatingHandler(services.Rating),
		Booking: handler.NewBookingHandler(services.Booking, cfg.UploadDir, cfg.AppBaseURL),
		Profile: handler.NewProfileHandler(services.User, services.Partner, cfg.UploadDir, cfg.AppBaseURL),
	}
}
