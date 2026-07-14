package auth

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword hashes a plain text password using bcrypt.
// Uses DefaultCost (10 rounds ~100ms) — deliberately slow to make brute force impractical.
// The hash is safe to store in the database — cannot be reversed.
func HashPassword(password string) (string, error) {
	var bytes []byte
	var err error
	bytes, err = bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(bytes), nil
}

// CheckPassword verifies a plain text password against a bcrypt hash.
// Extracts the salt from the stored hash, re-hashes the input, and compares.
// Returns nil if they match, error if they don't.
func CheckPassword(password string, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
