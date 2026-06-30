package config

import (
	"fmt"
	"os"
)

// Config holds all configuration values for the Hub.
// All values are read from environment variables — never hardcoded.
type Config struct {
	Port          string
	DatabaseURL   string
	RedisURL      string
	JWTSecret     string
	EncryptionKey string // 32-byte key for AES-256 token encryption at rest
}

// Load reads environment variables and returns a Config.
// If a required variable is missing, the app panics — it cannot start without it.
func Load() (*Config, error) {
	var cfg *Config
	cfg = &Config{
		Port:          getEnv("PORT", "8080"),
		DatabaseURL:   mustGetEnv("DATABASE_URL"),
		RedisURL:      mustGetEnv("REDIS_URL"),
		JWTSecret:     mustGetEnv("JWT_SECRET"),
		EncryptionKey: mustGetEnv("ENCRYPTION_KEY"),
	}
	return cfg, nil
}

func getEnv(key string, fallback string) string {
	var value string
	value = os.Getenv(key)
	if value != "" {
		return value
	}
	return fallback
}

func mustGetEnv(key string) string {
	var value string
	value = os.Getenv(key)
	if value == "" {
		// The app cannot start without required variables.
		panic(fmt.Sprintf("Required environment variable %q is not set", key))
	}
	return value
}
