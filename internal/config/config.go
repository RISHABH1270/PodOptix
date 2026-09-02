package config

import (
	"fmt"
	"os"
)

// Config holds all configuration values for the Hub.
// All values are read from environment variables — never hardcoded.
type Config struct {
	Port          string
	DatabaseURL   string // postgres://user:password@host:port/dbname?sslmode=disable
	RedisURL      string // redis://host:port
	JWTSecret     string // long random string — signs and verifies JWT tokens
	EncryptionKey string // exactly 32 bytes — AES-256 key for Prometheus token encryption at rest
}

// Load reads environment variables and returns a Config.
// Returns an error if any required variable is missing — app must not start without them.
func Load() (*Config, error) {
	databaseURL, err := mustGetEnv("DATABASE_URL")
	if err != nil {
		return nil, err
	}

	redisURL, err := mustGetEnv("REDIS_URL")
	if err != nil {
		return nil, err
	}

	jwtSecret, err := mustGetEnv("JWT_SECRET")
	if err != nil {
		return nil, err
	}

	encryptionKey, err := mustGetEnv("ENCRYPTION_KEY")
	if err != nil {
		return nil, err
	}

	return &Config{
		Port:          getEnv("PORT", "8080"),
		DatabaseURL:   databaseURL,
		RedisURL:      redisURL,
		JWTSecret:     jwtSecret,
		EncryptionKey: encryptionKey,
	}, nil
}

// getEnv reads an env variable — returns fallback if not set.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// mustGetEnv reads an env variable — returns an error if not set.
func mustGetEnv(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("required environment variable %q is not set", key)
	}
	return v, nil
}
