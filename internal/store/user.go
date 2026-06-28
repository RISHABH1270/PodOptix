package store

import (
	"context"
	"fmt"
	"time"

	"github.com/RISHABH1270/PodOptix/pkg/models"
)

// CreateUser inserts a new user into the database.
func (s *Store) CreateUser(ctx context.Context, u *models.User) error {
	query := `
		INSERT INTO users (user_id, email, password_hash, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := s.pool.Exec(ctx, query,
		u.UserID,
		u.Email,
		u.PasswordHash,
		u.CreatedAt,
		u.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// GetUserByEmail fetches a user by their email address.
// Used during login to verify credentials.
func (s *Store) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT user_id, email, password_hash, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	row := s.pool.QueryRow(ctx, query, email)

	var u models.User
	err := row.Scan(
		&u.UserID,
		&u.Email,
		&u.PasswordHash,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &u, nil
}

// UpdateUserPassword updates a user's password hash.
func (s *Store) UpdateUserPassword(ctx context.Context, userID string, passwordHash string) error {
	query := `
		UPDATE users
		SET password_hash = $1, updated_at = $2
		WHERE user_id = $3
	`
	_, err := s.pool.Exec(ctx, query, passwordHash, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("update user password: %w", err)
	}
	return nil
}
