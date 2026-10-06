package config

import (
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

// Config holds all runtime settings loaded from environment variables.
// Nothing is hard-coded — if you can't configure it from an env var, it doesn't go here.
type Config struct {
	Port          string
	DatabaseURL   string
	RedisURL      string
	JWTSecret     string
	EncryptionKey string
}

// AES-256-GCM key size (bytes). The auth package uses this cipher to encrypt
// Prometheus tokens at rest — a wrong length fails at RUNTIME inside the
// scheduler ("crypto/aes: invalid key size"), long after the pod is "healthy".
// Catch it at startup instead.
const encryptionKeyLength = 32

// Minimum length for the HS256 signing secret. HS256 technically accepts any
// length, but anything under 32 bytes is brute-forceable — refuse to start.
const jwtSecretMinLength = 32

// Load reads environment variables, validates them, and returns a Config.
// Any validation failure returns an error — caller (main) decides how to react.
func Load() (*Config, error) {
	databaseURL, err := mustGetEnv("DATABASE_URL")
	if err != nil {
		return nil, err
	}
	// Catch typos (bad scheme, missing host) with a config-flavored error now
	// instead of a cryptic pgx error later from store.EnsureDatabase.
	if _, err := pgx.ParseConfig(databaseURL); err != nil {
		return nil, fmt.Errorf("invalid DATABASE_URL: %w", err)
	}

	redisURL, err := mustGetEnv("REDIS_URL")
	if err != nil {
		return nil, err
	}
	if _, err := redis.ParseURL(redisURL); err != nil {
		return nil, fmt.Errorf("invalid REDIS_URL: %w", err)
	}

	jwtSecret, err := mustGetEnv("JWT_SECRET")
	if err != nil {
		return nil, err
	}
	if len(jwtSecret) < jwtSecretMinLength {
		return nil, fmt.Errorf("JWT_SECRET must be at least %d bytes (got %d) — weaker secrets are brute-forceable", jwtSecretMinLength, len(jwtSecret))
	}

	encryptionKey, err := mustGetEnv("ENCRYPTION_KEY")
	if err != nil {
		return nil, err
	}
	if len(encryptionKey) != encryptionKeyLength {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be exactly %d bytes for AES-256-GCM (got %d)", encryptionKeyLength, len(encryptionKey))
	}

	return &Config{
		Port:          getEnv("PORT", "8080"),
		DatabaseURL:   databaseURL,
		RedisURL:      redisURL,
		JWTSecret:     jwtSecret,
		EncryptionKey: encryptionKey,
	}, nil
}

// getEnv reads an env var and returns fallback if unset or empty.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// mustGetEnv reads an env var and returns an error if unset or empty.
func mustGetEnv(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("required environment variable %q is not set", key)
	}
	return v, nil
}

