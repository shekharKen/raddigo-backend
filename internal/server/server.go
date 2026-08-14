package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/raddigo/raddigo/internal/config"
)

// Server wraps an http.Server with graceful lifecycle helpers.
type Server struct {
	httpServer *http.Server
	logger     *slog.Logger
}

// New creates a Server bound to the configured address and handler.
func New(cfg config.Config, logger *slog.Logger, handler http.Handler) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:         cfg.ServerAddr,
			Handler:      handler,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
		},
		logger: logger,
	}
}

// Start begins serving and blocks until the server stops.
func (s *Server) Start() error {
	s.logger.Info("server starting", "addr", s.httpServer.Addr)
	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown gracefully stops the server within the provided context deadline.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("server shutting down")
	return s.httpServer.Shutdown(ctx)
}
