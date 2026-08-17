package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/raddigo/raddigo/internal/config"
	"github.com/raddigo/raddigo/internal/database"
	"github.com/raddigo/raddigo/internal/di"
	"github.com/raddigo/raddigo/internal/server"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := database.NewGORM(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	logger.Info("database connected")

	if err := database.Migrate(db); err != nil {
		return err
	}

	c := di.New(cfg, logger, db)

	router := server.NewRouter(
		logger,
		c.Handlers.Health,
		c.Handlers.Auth,
		c.Handlers.Partner,
		c.Handlers.Address,
	)
	srv := server.New(cfg, logger, router)

	// Run the server and capture a startup failure.
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- srv.Start()
	}()

	// Wait for an interrupt signal or a server error.
	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		stop()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
