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
// Connects to the default "postgres" admin database first since the target may not exist yet.
//
// Uses the ORIGINAL parsed connection config (mutated to target the "postgres" DB) so
// TLS settings, cert paths, connect_timeout, and anything else the operator put in
// DATABASE_URL are preserved for the admin connection too.
func EnsureDatabase(databaseURL string) error {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("parse database url: %w", err)
	}
	dbName := cfg.ConnConfig.Database

	// Copy the parsed config and point it at the "postgres" admin DB.
	// Copy() is defensive — ensures we never mutate the caller's cfg struct.
	adminCfg := cfg.ConnConfig.Copy()
	adminCfg.Database = "postgres"

	// Startup timeouts — if PG is unreachable, fail fast with a clean error instead
	// of hanging until Kubernetes' livenessProbe kills the pod minutes later.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := pgx.ConnectConfig(ctx, adminCfg)
	if err != nil {
		return fmt.Errorf("connect to postgres admin db: %w", err)
	}
	defer conn.Close(context.Background())

	_, err = conn.Exec(ctx, "CREATE DATABASE "+dbName)
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

// SyncSchema applies pending migration files from migrations/ in sequence.
// If a previous run crashed mid-migration, the schema_migrations table will be
// in a "dirty" state. We DO NOT auto-fix dirty state — Force() only updates
// metadata, it doesn't inspect the actual schema. Auto-forcing to the current
// (failed) version would silently mark an unfinished migration as applied,
// leaving the schema partially migrated with no warning. That's worse than a
// loud failure.
//
// On dirty state: SyncSchema returns an error pointing the operator at the
// migrate CLI so they can inspect, roll back manually, and force to the last
// known-good version before restarting the service.
func SyncSchema(databaseURL string) error {
	m, err := migrate.New("file://migrations", databaseURL)
	if err != nil {
		return fmt.Errorf("create schema syncer: %w", err)
	}

	// Pre-flight: is the migrations table in a dirty state from a previous crash?
	version, dirty, vErr := m.Version()
	if vErr != nil && vErr != migrate.ErrNilVersion {
		return fmt.Errorf("read current migration version: %w", vErr)
	}
	if dirty {
		return fmt.Errorf(
			"migrations are in a DIRTY state at version %d (a previous migration crashed).\n"+
				"  Inspect the schema manually, then force to the last known-good version:\n"+
				"    migrate -database $DATABASE_URL -path migrations force %d\n"+
				"  Then restart the service",
			version, version-1,
		)
	}

	// Clean state — apply any pending migrations.
	err = m.Up()
	if err == migrate.ErrNoChange {
		return nil
	}
	if err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

// ── Step 3: New ───────────────────────────────────────────────────────────────

// New opens the connection pool with 10/2 max/min tuning and verifies connectivity
// with Ping. Startup is capped at 30s so an unreachable DB fails fast with a clean
// error instead of hanging until K8s' livenessProbe fires.
func New(databaseURL string) (*Store, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	config.MaxConns = 10
	config.MinConns = 2
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close() // don't leak the half-open pool on a ping failure
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
