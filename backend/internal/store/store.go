// Package store provides the Postgres connection pool, embedded migrations,
// and query helpers used by the rest of the backend.
package store

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Store wraps a pgx connection pool.
type Store struct {
	Pool *pgxpool.Pool
}

// Open connects to Postgres and runs pending migrations.
func Open(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connecting to postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging postgres: %w", err)
	}
	if err := runMigrations(databaseURL); err != nil {
		pool.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}
	return &Store{Pool: pool}, nil
}

func runMigrations(databaseURL string) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("loading migration source: %w", err)
	}
	// golang-migrate's pgx/v5 driver registers under the "pgx5" scheme.
	migrateURL := databaseURL
	if idx := strings.Index(migrateURL, "://"); idx >= 0 {
		migrateURL = "pgx5" + migrateURL[idx:]
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, migrateURL)
	if err != nil {
		return fmt.Errorf("initialising migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

// Close closes the underlying connection pool.
func (s *Store) Close() {
	s.Pool.Close()
}

// Ping checks database reachability.
func (s *Store) Ping(ctx context.Context) error {
	return s.Pool.Ping(ctx)
}
