package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/seraph/triage/apps/api/internal/store/db"
)

//go:embed migrations/*.sql
var EmbedMigrations embed.FS

// Store provides database operations, transaction management, and tenant isolation.
type Store struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	rawURL  string
}

// NewStore initializes a pgx connection pool and returns a Store.
func NewStore(ctx context.Context, databaseURL string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	cfg.MaxConns = 25
	cfg.MinConns = 5
	cfg.MaxConnLifetime = 1 * time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	queries := db.New(pool)

	return &Store{
		pool:    pool,
		queries: queries,
		rawURL:  databaseURL,
	}, nil
}

// Close closes the pgx pool.
func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

// Ping checks database connectivity.
func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// Queries returns the raw queries handler.
func (s *Store) Queries() *db.Queries {
	return s.queries
}

// Pool returns the underlying pgx pool.
func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}

// Migrate applies all pending goose migrations using the embedded filesystem.
func (s *Store) Migrate(ctx context.Context) error {
	sqlDB, err := sql.Open("pgx", s.rawURL)
	if err != nil {
		return fmt.Errorf("failed to open sql db for migrations: %w", err)
	}
	defer sqlDB.Close()

	goose.SetBaseFS(EmbedMigrations)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	if err := goose.UpContext(ctx, sqlDB, "migrations"); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	return nil
}

// WithOrgTx executes a transaction scoped to a specific organization.
// It sets the session-local 'app.org_id' setting so Postgres Row-Level Security
// enforces strict tenant boundaries at the database engine level.
func (s *Store) WithOrgTx(ctx context.Context, orgID pgtype.UUID, fn func(queries *db.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	orgIDStr := fmt.Sprintf("%x-%x-%x-%x-%x", orgID.Bytes[0:4], orgID.Bytes[4:6], orgID.Bytes[6:8], orgID.Bytes[8:10], orgID.Bytes[10:16])

	// Enforce session setting for RLS policies
	_, err = tx.Exec(ctx, "SELECT set_config('app.org_id', $1, true)", orgIDStr)
	if err != nil {
		return fmt.Errorf("failed to set tenant session setting: %w", err)
	}

	q := s.queries.WithTx(tx)
	if err := fn(q); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
