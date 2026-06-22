package store

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store holds the PostgreSQL connection pool.
// All database operations go through this.
type Store struct {
	pool *pgxpool.Pool
}

// New connects to PostgreSQL and returns a Store with a connection pool.
func New(databaseURL string) (*Store, error) {
	// configure the connection pool
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	config.MaxConns = 10                      // max 10 connections open at once
	config.MinConns = 2                       // always keep 2 connections ready
	config.MaxConnLifetime = time.Hour        // refresh connections after 1 hour
	config.MaxConnIdleTime = 30 * time.Minute // close idle connections after 30 min

	// open the pool
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	// verify the connection is actually working
	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &Store{pool: pool}, nil
}

// Close shuts down the connection pool gracefully.
func (s *Store) Close() {
	s.pool.Close()
}

// SyncSchema runs all SQL files from the migrations/ folder on startup.
// Safe to run multiple times — skips already applied schemas (IF NOT EXISTS).
// If the database is in a dirty state from a previous failed run, it auto-fixes it.
func SyncSchema(databaseURL string) error {
	var m *migrate.Migrate
	var err error

	m, err = migrate.New("file://migrations", databaseURL)
	if err != nil {
		return fmt.Errorf("create schema syncer: %w", err)
	}

	err = m.Up()
	if err == migrate.ErrNoChange {
		// already up to date — nothing to do
		return nil
	}
	if err != nil {
		// check if database is dirty from a previous failed migration
		// auto-fix by forcing the version and retrying
		version, _, vErr := m.Version()
		if vErr == nil && version > 0 {
			if fErr := m.Force(int(version)); fErr == nil {
				// retry after forcing clean state
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
