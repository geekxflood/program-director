package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/geekxflood/program-director/internal/config"
)

// PostgresDB implements DB interface for PostgreSQL
type PostgresDB struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewPostgres creates a new PostgreSQL connection
func NewPostgres(ctx context.Context, cfg *config.PostgresConfig, logger *slog.Logger) (*PostgresDB, error) {
	dsn := cfg.DSN()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	// Test connection
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	logger.Info("connected to PostgreSQL",
		"host", cfg.Host,
		"database", cfg.Database,
	)

	return &PostgresDB{
		db:     db,
		logger: logger,
	}, nil
}

// Close closes the database connection
func (p *PostgresDB) Close() error {
	return p.db.Close()
}

// Ping checks if the database connection is alive
func (p *PostgresDB) Ping(ctx context.Context) error {
	return p.db.PingContext(ctx)
}

// BeginTx starts a new transaction
func (p *PostgresDB) BeginTx(ctx context.Context) (Tx, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &PostgresTx{tx: tx}, nil
}

// Query executes a query that returns rows
func (p *PostgresDB) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return p.db.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns a single row
func (p *PostgresDB) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return p.db.QueryRowContext(ctx, query, args...)
}

// Exec executes a query that doesn't return rows
func (p *PostgresDB) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return p.db.ExecContext(ctx, query, args...)
}

// Migrate runs all pending migrations
func (p *PostgresDB) Migrate(ctx context.Context) error {
	p.logger.Info("running database migrations")

	// Create migrations table
	if err := createMigrationsTable(ctx, p, "postgres"); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get applied migrations
	applied, err := getAppliedMigrations(ctx, p)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Load migrations
	migrations, err := loadMigrations("postgres")
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	// Apply pending migrations
	for _, m := range migrations {
		if applied[m.Version] {
			p.logger.Debug("migration already applied", "version", m.Version, "name", m.Name)
			continue
		}

		p.logger.Info("applying migration", "version", m.Version, "name", m.Name)

		// Execute migration in transaction
		tx, err := p.BeginTx(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		if _, err := tx.Exec(ctx, m.SQL); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to execute migration %d: %w", m.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %d: %w", m.Version, err)
		}

		// Record migration
		if err := recordMigration(ctx, p, m); err != nil {
			return fmt.Errorf("failed to record migration %d: %w", m.Version, err)
		}

		p.logger.Info("migration applied successfully", "version", m.Version, "name", m.Name)
	}

	return nil
}

// PostgresTx wraps sql.Tx to implement Tx interface
type PostgresTx struct {
	tx *sql.Tx
}

func (t *PostgresTx) Commit() error {
	return t.tx.Commit()
}

func (t *PostgresTx) Rollback() error {
	return t.tx.Rollback()
}

func (t *PostgresTx) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return t.tx.QueryContext(ctx, query, args...)
}

func (t *PostgresTx) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return t.tx.QueryRowContext(ctx, query, args...)
}

func (t *PostgresTx) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return t.tx.ExecContext(ctx, query, args...)
}
