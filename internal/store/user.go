package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/RISHABH1270/PodOptix/pkg/models"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrEmailAlreadyRegistered is returned by CreateUser when the email is already
// in use. Handler maps to 409 Conflict. For a public-facing SaaS this would be
// a user-enumeration vulnerability — PodOptix is operator-facing, so the clear
// message is worth the trade-off.
var ErrEmailAlreadyRegistered = errors.New("email already registered")

// normalizeEmail is the single source of truth for how we treat emails as
// identities. Case + leading/trailing whitespace are stripped so that
// "admin@example.com", "Admin@Example.com", and "  ADMIN@example.com  "
// all map to the same stored row. Without this, the UNIQUE constraint on
// users.email would be trivially bypassed with case variants — an actual
// impersonation vector.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// CreateUser inserts a new user. Email is normalized in place so the stored
// value and the value the caller sees after this call are identical.
// Returns ErrEmailAlreadyRegistered if the email is already taken.
func (s *Store) CreateUser(ctx context.Context, u *models.User) error {
	u.Email = normalizeEmail(u.Email)
	query := `
		INSERT INTO users (user_id, email, password_hash, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
	`
	_, err := s.pool.Exec(ctx, query, u.UserID, u.Email, u.PasswordHash, u.CreatedAt)
	if err != nil {
		// SQLSTATE 23505 = unique_violation. users.email is the only UNIQUE
		// constraint on the table, so a hit means the email is taken.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrEmailAlreadyRegistered
		}
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// GetUserByEmail fetches a user by email. Email is normalized before lookup so
// mixed-case or whitespace-padded login attempts hit the same row as the
// stored (normalized) email. Used during login to verify credentials — the
// caller treats "not found" and "DB error" identically to avoid leaking
// which emails are registered.
func (s *Store) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	email = normalizeEmail(email)
	query := `SELECT user_id, email, password_hash, created_at, updated_at FROM users WHERE email = $1`
	var u models.User
	err := s.pool.QueryRow(ctx, query, email).Scan(
		&u.UserID, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &u, nil
}
