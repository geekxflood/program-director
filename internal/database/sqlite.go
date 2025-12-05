package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"github.com/geekxflood/program-director/internal/config"
)

// SQLiteDB implements DB interface for SQLite
type SQLiteDB struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewSQLite creates a new SQLite connection
func NewSQLite(ctx context.Context, cfg *config.SQLiteConfig, logger *slog.Logger) (*SQLiteDB, error) {
	dbPath := cfg.Path
	if dbPath == "" {
		dbPath = "./data/program-director.db"
	}

	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open connection with WAL mode and foreign keys enabled
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=ON&_busy_timeout=5000", dbPath)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite connection: %w", err)
	}

	// SQLite works best with single connection for write operations
	db.SetMaxOpenConns(1)

	// Test connection
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping sqlite: %w", err)
	}

	logger.Info("connected to SQLite",
		"path", dbPath,
	)

	return &SQLiteDB{
		db:     db,
		logger: logger,
	}, nil
}

// Close closes the database connection
func (s *SQLiteDB) Close() error {
	return s.db.Close()
}

// Ping checks if the database connection is alive
func (s *SQLiteDB) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// BeginTx starts a new transaction
func (s *SQLiteDB) BeginTx(ctx context.Context) (Tx, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &SQLiteTx{tx: tx}, nil
}

// Query executes a query that returns rows
func (s *SQLiteDB) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	// Convert $1, $2 style placeholders to ? for SQLite
	query = convertPlaceholders(query)
	return s.db.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns a single row
func (s *SQLiteDB) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	query = convertPlaceholders(query)
	return s.db.QueryRowContext(ctx, query, args...)
}

// Exec executes a query that doesn't return rows
func (s *SQLiteDB) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	query = convertPlaceholders(query)
	return s.db.ExecContext(ctx, query, args...)
}

// Migrate runs all pending migrations
func (s *SQLiteDB) Migrate(ctx context.Context) error {
	s.logger.Info("running database migrations")

	// Create migrations table
	if err := createMigrationsTable(ctx, s, "sqlite"); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get applied migrations
	applied, err := getAppliedMigrations(ctx, s)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Load migrations
	migrations, err := loadMigrations("sqlite")
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	// Apply pending migrations
	for _, m := range migrations {
		if applied[m.Version] {
			s.logger.Debug("migration already applied", "version", m.Version, "name", m.Name)
			continue
		}

		s.logger.Info("applying migration", "version", m.Version, "name", m.Name)

		// Execute migration in transaction
		tx, err := s.BeginTx(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		// Convert placeholders in migration SQL
		migrationSQL := convertPlaceholders(m.SQL)
		if _, err := tx.Exec(ctx, migrationSQL); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to execute migration %d: %w", m.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %d: %w", m.Version, err)
		}

		// Record migration
		if err := recordMigration(ctx, s, m); err != nil {
			return fmt.Errorf("failed to record migration %d: %w", m.Version, err)
		}

		s.logger.Info("migration applied successfully", "version", m.Version, "name", m.Name)
	}

	return nil
}

// SQLiteTx wraps sql.Tx to implement Tx interface
type SQLiteTx struct {
	tx *sql.Tx
}

func (t *SQLiteTx) Commit() error {
	return t.tx.Commit()
}

func (t *SQLiteTx) Rollback() error {
	return t.tx.Rollback()
}

func (t *SQLiteTx) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	query = convertPlaceholders(query)
	return t.tx.QueryContext(ctx, query, args...)
}

func (t *SQLiteTx) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	query = convertPlaceholders(query)
	return t.tx.QueryRowContext(ctx, query, args...)
}

func (t *SQLiteTx) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	query = convertPlaceholders(query)
	return t.tx.ExecContext(ctx, query, args...)
}

// convertPlaceholders converts PostgreSQL-style $1, $2 placeholders to SQLite ? placeholders
func convertPlaceholders(query string) string {
	result := make([]byte, 0, len(query))
	i := 0
	for i < len(query) {
		if query[i] == '$' && i+1 < len(query) && query[i+1] >= '0' && query[i+1] <= '9' {
			// Skip the $ and all following digits
			result = append(result, '?')
			i++
			for i < len(query) && query[i] >= '0' && query[i] <= '9' {
				i++
			}
		} else {
			result = append(result, query[i])
			i++
		}
	}
	return string(result)
}
