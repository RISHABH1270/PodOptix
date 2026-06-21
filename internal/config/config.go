package config

import (
	"fmt"
	"os"
)

// Config holds all configuration values for the Hub.
// All values are read from environment variables — never hardcoded.
type Config struct {
	Port        string // which port the HTTP server listens on
	DatabaseURL string // PostgreSQL connection string
	RedisURL    string // Redis connection string
	JWTSecret   string // secret key used to sign JWT tokens
}

// Load reads environment variables and returns a Config.
// If a required variable is missing, the app panics — it cannot start without it.
func Load() (*Config, error) {
	cfg := &Config{
		Port:        getEnv("PORT", "8080"), // defaults to 8080 if not set
		DatabaseURL: mustGetEnv("DATABASE_URL"),
		RedisURL:    mustGetEnv("REDIS_URL"),
		JWTSecret:   mustGetEnv("JWT_SECRET"),
	}
	return cfg, nil
}

// getEnv reads an env variable — returns fallback if not set.
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// mustGetEnv reads an env variable — panics if not set.
// The app cannot start without required variables.
func mustGetEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic(fmt.Sprintf("required environment variable %q is not set", key))
	}
	return value
}
