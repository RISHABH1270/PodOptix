package auth

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword hashes a plain text password using bcrypt (cost=10, ~100ms).
// Deliberately slow — makes brute force impractical. Hash is safe to store in DB.
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(b), nil
}

// CheckPassword verifies a plain text password against a bcrypt hash.
// Returns nil if they match, error if they don't.
func CheckPassword(password string, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
