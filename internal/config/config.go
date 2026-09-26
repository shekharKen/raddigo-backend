package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds runtime configuration for the application.
type Config struct {
	ServerAddr               string
	ReadTimeout              time.Duration
	WriteTimeout             time.Duration
	IdleTimeout              time.Duration
	ShutdownTimeout          time.Duration
	DatabaseURL              string
	AppBaseURL               string
	PublicDir                string
	UploadDir                string
	JWTSecret                string
	AccessTokenTTL           time.Duration
	RefreshTokenTTL          time.Duration
	SlotDuration             time.Duration
	DevOTP                   string
	MonthlySubscriptionPrice float64
	AnnualSubscriptionPrice  float64
	FirebaseCredentialsFile  string
	ReminderCheckInterval    time.Duration
	ReminderLeadMinutes      int
	SMTPHost                 string
	SMTPPort                 int
	SMTPUsername             string
	SMTPPassword             string
	SMTPFromAddress          string
	SMTPFromName             string
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
		AccessTokenTTL:  getDuration("ACCESS_TOKEN_TTL", 24*time.Hour),
		RefreshTokenTTL: getDuration("REFRESH_TOKEN_TTL", 7*24*time.Hour),
		SlotDuration:    getDuration("SLOT_DURATION", 30*time.Minute),
		// DevOTP, when set, is used as the fixed verification OTP instead of a
		// random one. Intended for local development only; leave unset in production.
		DevOTP:                   getEnv("DEV_OTP", ""),
		MonthlySubscriptionPrice: getFloat("MONTHLY_SUBSCRIPTION_PRICE", 499),
		AnnualSubscriptionPrice:  getFloat("ANNUAL_SUBSCRIPTION_PRICE", 4999),
		// FirebaseCredentialsFile, when unset, falls back to a log-only notifier.
		FirebaseCredentialsFile: getEnv("GOOGLE_APPLICATION_CREDENTIALS", ""),
		ReminderCheckInterval:   getDuration("REMINDER_CHECK_INTERVAL", time.Minute),
		ReminderLeadMinutes:     getInt("REMINDER_LEAD_MINUTES", 30),
		// SMTPHost, when unset, falls back to a log-only mailer.
		SMTPHost:        getEnv("SMTP_HOST", ""),
		SMTPPort:        getInt("SMTP_PORT", 587),
		SMTPUsername:    getEnv("SMTP_USERNAME", ""),
		SMTPPassword:    getEnv("SMTP_PASSWORD", ""),
		SMTPFromAddress: getEnv("SMTP_FROM_ADDRESS", "support@raddigo.com"),
		SMTPFromName:    getEnv("SMTP_FROM_NAME", "Raddigo"),
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

func getFloat(key string, fallback float64) float64 {
	if v, ok := os.LookupEnv(key); ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func getInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
