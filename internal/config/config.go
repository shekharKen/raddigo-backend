package config

import (
	"os"
	"time"
)

// Config holds runtime configuration for the application.
type Config struct {
	ServerAddr      string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	DatabaseURL     string
	AppBaseURL      string
	PublicDir       string
	UploadDir       string
	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

// Load builds a Config from environment variables, falling back to sensible
// defaults when a variable is unset or invalid.
func Load() Config {
	return Config{
		ServerAddr:      getEnv("SERVER_ADDR", ":8080"),
		ReadTimeout:     getDuration("READ_TIMEOUT", 10*time.Second),
		WriteTimeout:    getDuration("WRITE_TIMEOUT", 10*time.Second),
		IdleTimeout:     getDuration("IDLE_TIMEOUT", 60*time.Second),
		ShutdownTimeout: getDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/raddigo?sslmode=disable"),
		AppBaseURL:      getEnv("APP_BASE_URL", "http://localhost:8080"),
		PublicDir:       getEnv("PUBLIC_DIR", "./public"),
		UploadDir:       getEnv("UPLOAD_DIR", "./public/uploads"),
		JWTSecret:       getEnv("JWT_SECRET", "dev-secret-change-me"),
		AccessTokenTTL:  getDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL: getDuration("REFRESH_TOKEN_TTL", 720*time.Hour),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
