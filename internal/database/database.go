package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"

	"github.com/geekxflood/program-director/internal/config"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DB is the interface for database operations
type DB interface {
	// Core operations
	Close() error
	Ping(ctx context.Context) error

	// Transaction support
	BeginTx(ctx context.Context) (Tx, error)

	// Query operations
	Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row
	Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error)

	// Migration
	Migrate(ctx context.Context) error
}

// Tx represents a database transaction
type Tx interface {
	Commit() error
	Rollback() error
	Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row
	Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// New creates a new database connection based on configuration
func New(ctx context.Context, cfg *config.DatabaseConfig, logger *slog.Logger) (DB, error) {
	switch cfg.Driver {
	case "postgres":
		return NewPostgres(ctx, &cfg.Postgres, logger)
	case "sqlite":
		return NewSQLite(ctx, &cfg.SQLite, logger)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}
}

// loadMigrations reads all SQL migration files
func loadMigrations(driver string) ([]Migration, error) {
	var migrations []Migration

	// Read migrations from embedded filesystem
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}

		// Parse migration version and name
		// Expected format: 001_create_media_table.sql
		parts := strings.SplitN(strings.TrimSuffix(name, ".sql"), "_", 2)
		if len(parts) != 2 {
			continue
		}

		var version int
		if _, err := fmt.Sscanf(parts[0], "%d", &version); err != nil {
			continue
		}

		content, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return nil, fmt.Errorf("failed to read migration %s: %w", name, err)
		}

		// Adapt SQL for different databases
		sql := string(content)
		sql = adaptSQL(sql, driver)

		migrations = append(migrations, Migration{
			Version: version,
			Name:    parts[1],
			SQL:     sql,
		})
	}

	// Sort by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// Migration represents a database migration
type Migration struct {
	Version int
	Name    string
	SQL     string
}

// adaptSQL converts PostgreSQL-specific SQL to SQLite where needed
func adaptSQL(sql string, driver string) string {
	if driver == "sqlite" {
		// Convert JSONB to TEXT for SQLite
		sql = strings.ReplaceAll(sql, "JSONB", "TEXT")
		sql = strings.ReplaceAll(sql, "jsonb", "TEXT")

		// Convert SERIAL to INTEGER for SQLite (auto-increment is implicit)
		sql = strings.ReplaceAll(sql, "SERIAL PRIMARY KEY", "INTEGER PRIMARY KEY AUTOINCREMENT")
		sql = strings.ReplaceAll(sql, "BIGSERIAL PRIMARY KEY", "INTEGER PRIMARY KEY AUTOINCREMENT")

		// Remove IF NOT EXISTS from CREATE INDEX (SQLite supports it differently)
		// Actually SQLite does support IF NOT EXISTS, so we're good

		// Convert timestamp defaults
		sql = strings.ReplaceAll(sql, "CURRENT_TIMESTAMP", "CURRENT_TIMESTAMP")
	}
	return sql
}

// createMigrationsTable creates the schema_migrations table if it doesn't exist
func createMigrationsTable(ctx context.Context, db DB, driver string) error {
	var createSQL string
	if driver == "sqlite" {
		createSQL = `
			CREATE TABLE IF NOT EXISTS schema_migrations (
				version INTEGER PRIMARY KEY,
				name TEXT NOT NULL,
				applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)
		`
	} else {
		createSQL = `
			CREATE TABLE IF NOT EXISTS schema_migrations (
				version INTEGER PRIMARY KEY,
				name TEXT NOT NULL,
				applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)
		`
	}

	_, err := db.Exec(ctx, createSQL)
	return err
}

// getAppliedMigrations returns the versions of already applied migrations
func getAppliedMigrations(ctx context.Context, db DB) (map[int]bool, error) {
	applied := make(map[int]bool)

	rows, err := db.Query(ctx, "SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}

	return applied, rows.Err()
}

// recordMigration records that a migration was applied
func recordMigration(ctx context.Context, db DB, m Migration) error {
	_, err := db.Exec(ctx,
		"INSERT INTO schema_migrations (version, name) VALUES ($1, $2)",
		m.Version, m.Name,
	)
	return err
}
