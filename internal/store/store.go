package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store holds a pointer to the PostgreSQL connection pool.
// All store operations (cluster, recommendation, user) share this single pool.
type Store struct {
	pool *pgxpool.Pool
}

// ── Step 1: EnsureDatabase ────────────────────────────────────────────────────

// EnsureDatabase creates the target database if it does not already exist.
// Connects to the default "postgres" database first since the target may not exist yet.
func EnsureDatabase(databaseURL string) error {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("parse database url: %w", err)
	}

	dbName := cfg.ConnConfig.Database

	adminURL := fmt.Sprintf("postgres://%s:%s@%s:%d/postgres?sslmode=disable",
		cfg.ConnConfig.User,
		cfg.ConnConfig.Password,
		cfg.ConnConfig.Host,
		cfg.ConnConfig.Port,
	)

	conn, err := pgx.Connect(context.Background(), adminURL)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer conn.Close(context.Background())

	_, err = conn.Exec(context.Background(), "CREATE DATABASE "+dbName)
	if err != nil {
		// 42P04 = "database already exists" — stable across PostgreSQL versions and locales
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "42P04" {
			return nil
		}
		return fmt.Errorf("create database: %w", err)
	}

	return nil
}

// ── Step 2: SyncSchema ────────────────────────────────────────────────────────

// SyncSchema runs all SQL migration files from migrations/ in sequence.
// Skips already applied migrations. Auto-fixes dirty state from a previous crash.
func SyncSchema(databaseURL string) error {
	m, err := migrate.New("file://migrations", databaseURL)
	if err != nil {
		return fmt.Errorf("create schema syncer: %w", err)
	}

	err = m.Up()
	if err == migrate.ErrNoChange {
		return nil
	}
	if err != nil {
		version, _, vErr := m.Version()
		if vErr == nil && version > 0 {
			if fErr := m.Force(int(version)); fErr == nil {
				if rErr := m.Up(); rErr != nil && rErr != migrate.ErrNoChange {
					return fmt.Errorf("sync schema after force: %w", rErr)
				}
				return nil
			}
		}
		return fmt.Errorf("sync schema: %w", err)
	}

	return nil
}

// ── Step 3: New ───────────────────────────────────────────────────────────────

// New creates the connection pool and returns *Store — a pointer to the Store
// which holds the address of the pool allocated in heap memory.
// Ping() is called on startup to fail fast if the database is unreachable.
func New(databaseURL string) (*Store, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	config.MaxConns = 10
	config.MinConns = 2
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &Store{pool: pool}, nil
}

// ── Step 4: Ping ──────────────────────────────────────────────────────────────

// Ping verifies the database connection is alive — used by /readyz readiness probe.
func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// ── Step 5: Close ─────────────────────────────────────────────────────────────

// Close shuts down the connection pool gracefully — called via defer in main.
func (s *Store) Close() {
	s.pool.Close()
}
