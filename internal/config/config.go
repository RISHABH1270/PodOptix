package config

import (
	"fmt"
	"os"
)

// All values are read from environment variables — never hardcoded.
type Config struct {
	Port          string
	DatabaseURL   string 
	RedisURL      string 
	JWTSecret     string 
	EncryptionKey string 
}

// Load reads environment variables and returns a Config struct pointer.
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
