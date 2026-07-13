package config

import (
	"fmt"
	"os"
)

// Config holds all configuration values for the Hub.
// All values are read from environment variables — never hardcoded.
// Values differ per environment: local (.env file), staging/production (Kubernetes Secrets).
type Config struct {
	Port          string
	DatabaseURL   string
	RedisURL      string
	JWTSecret     string
	EncryptionKey string // 32-byte key for AES-256 token encryption at rest
}

// Load reads environment variables and returns a Config.
// Returns an error if any required variable is missing — the app must not start without them.
func Load() (*Config, error) {
	var err error

	var databaseURL string
	databaseURL, err = mustGetEnv("DATABASE_URL")
	if err != nil {
		return nil, err
	}

	var redisURL string
	redisURL, err = mustGetEnv("REDIS_URL")
	if err != nil {
		return nil, err
	}

	var jwtSecret string
	jwtSecret, err = mustGetEnv("JWT_SECRET")
	if err != nil {
		return nil, err
	}

	var encryptionKey string
	encryptionKey, err = mustGetEnv("ENCRYPTION_KEY")
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
func getEnv(key string, fallback string) string {
	var value string
	value = os.Getenv(key)
	if value != "" {
		return value
	}
	return fallback
}

// mustGetEnv reads an env variable — returns an error if not set.
func mustGetEnv(key string) (string, error) {
	var value string
	value = os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("required environment variable %q is not set", key)
	}
	return value, nil
}
