package models

import "time"

// User represents a PodOptix dashboard user.
type User struct {
	UserID       string    `json:"user_id"    db:"user_id"`
	Email        string    `json:"email"      db:"email"`
	PasswordHash string    `json:"-"          db:"password_hash"` // never exposed in API response
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}
