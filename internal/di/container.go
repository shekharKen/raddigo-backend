package di

import (
	"log/slog"

	"gorm.io/gorm"

	"github.com/raddigo/raddigo/internal/config"
	"github.com/raddigo/raddigo/internal/handler"
	"github.com/raddigo/raddigo/internal/mailer"
	"github.com/raddigo/raddigo/internal/repository"
	"github.com/raddigo/raddigo/internal/service"
)

// Repositories groups the data-access layer.
type Repositories struct {
	User    repository.UserRepository
	Ragman  repository.RagmanRepository
	Address repository.AddressRepository
}

// Services groups the business-logic layer.
type Services struct {
	User    *service.UserService
	Ragman  *service.RagmanService
	Address *service.AddressService
}

// Handlers groups the HTTP layer.
type Handlers struct {
	Health  *handler.HealthHandler
	Auth    *handler.AuthHandler
	Ragman  *handler.RagmanHandler
	Address *handler.AddressHandler
}

// Container holds the fully wired application dependencies.
type Container struct {
	Repositories Repositories
	Services     Services
	Handlers     Handlers
}

// New builds the dependency graph layer by layer: repositories, then services,
// then handlers.
func New(cfg config.Config, logger *slog.Logger, db *gorm.DB) *Container {
	repos := buildRepositories(db)
	mail := mailer.NewLogMailer(logger)
	services := buildServices(cfg, repos, mail)
	handlers := buildHandlers(services)

	return &Container{
		Repositories: repos,
		Services:     services,
		Handlers:     handlers,
	}
}

func buildRepositories(db *gorm.DB) Repositories {
	return Repositories{
		User:    repository.NewGormUserRepository(db),
		Ragman:  repository.NewGormRagmanRepository(db),
		Address: repository.NewGormAddressRepository(db),
	}
}

func buildServices(cfg config.Config, repos Repositories, mail mailer.Mailer) Services {
	return Services{
		User:    service.NewUserService(repos.User, mail, cfg.AppBaseURL),
		Ragman:  service.NewRagmanService(repos.Ragman, mail, cfg.AppBaseURL),
		Address: service.NewAddressService(repos.Address),
	}
}

func buildHandlers(services Services) Handlers {
	return Handlers{
		Health:  handler.NewHealthHandler(),
		Auth:    handler.NewAuthHandler(services.User),
		Ragman:  handler.NewRagmanHandler(services.Ragman),
		Address: handler.NewAddressHandler(services.Address),
	}
}
