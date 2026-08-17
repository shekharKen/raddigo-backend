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
}

// Services groups the business-logic layer.
type Services struct {
	User    *service.UserService
	Partner *service.PartnerService
	Address *service.AddressService
	Rating  *service.RatingService
	Auth    *service.AuthService
}

// Handlers groups the HTTP layer.
type Handlers struct {
	Health  *handler.HealthHandler
	Auth    *handler.AuthHandler
	Partner *handler.PartnerHandler
	Address *handler.AddressHandler
	Rating  *handler.RatingHandler
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
	handlers := buildHandlers(services)

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
	}
}

func buildServices(cfg config.Config, repos Repositories, mail mailer.Mailer, tokens *auth.TokenService) Services {
	return Services{
		User:    service.NewUserService(repos.User, mail, cfg.AppBaseURL),
		Partner: service.NewPartnerService(repos.Partner, repos.Rating, mail, cfg.AppBaseURL),
		Address: service.NewAddressService(repos.Address),
		Rating:  service.NewRatingService(repos.Rating),
		Auth:    service.NewAuthService(repos.User, repos.Partner, tokens),
	}
}

func buildHandlers(services Services) Handlers {
	return Handlers{
		Health:  handler.NewHealthHandler(),
		Auth:    handler.NewAuthHandler(services.User, services.Auth),
		Partner: handler.NewPartnerHandler(services.Partner, services.Auth),
		Address: handler.NewAddressHandler(services.Address),
		Rating:  handler.NewRatingHandler(services.Rating),
	}
}
