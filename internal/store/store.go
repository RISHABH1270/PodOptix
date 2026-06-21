package store

import (
	"context"
	"fmt"
	"time"

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

	config.MaxConns = 10               // max 10 connections open at once
	config.MinConns = 2                // always keep 2 connections ready
	config.MaxConnLifetime = time.Hour // refresh connections after 1 hour
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
