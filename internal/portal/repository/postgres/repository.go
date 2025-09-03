package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/songzhibin97/stargate/pkg/portal"
)

// Repository implements the portal.Repository interface using PostgreSQL
type Repository struct {
	db             *sql.DB
	dsn            string
	maxOpenConns   int
	maxIdleConns   int
	connMaxLifetime time.Duration
	migrationPath  string
}

// Config holds the configuration for PostgreSQL repository
type Config struct {
	DSN             string        `yaml:"dsn" json:"dsn"`
	MaxOpenConns    int           `yaml:"max_open_conns" json:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns" json:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" json:"conn_max_lifetime"`
	MigrationPath   string        `yaml:"migration_path" json:"migration_path"`
}

// DefaultConfig returns a default PostgreSQL configuration
func DefaultConfig() *Config {
	return &Config{
		DSN:             "postgres://postgres:password@localhost:5432/stargate?sslmode=disable",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		MigrationPath:   "file://internal/portal/repository/postgres/migrations",
	}
}

// NewRepository creates a new PostgreSQL repository
func NewRepository(config *Config) (*Repository, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Open database connection
	db, err := sql.Open("postgres", config.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	repo := &Repository{
		db:              db,
		dsn:             config.DSN,
		maxOpenConns:    config.MaxOpenConns,
		maxIdleConns:    config.MaxIdleConns,
		connMaxLifetime: config.ConnMaxLifetime,
		migrationPath:   config.MigrationPath,
	}

	return repo, nil
}

// Migrate runs database migrations
func (r *Repository) Migrate() error {
	driver, err := postgres.WithInstance(r.db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(r.migrationPath, "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// Health returns the health status of the repository
func (r *Repository) Health(ctx context.Context) portal.HealthStatus {
	status := "healthy"
	message := "PostgreSQL repository is operational"
	details := map[string]interface{}{
		"database_type":     "postgresql",
		"max_open_conns":    r.maxOpenConns,
		"max_idle_conns":    r.maxIdleConns,
		"conn_max_lifetime": r.connMaxLifetime.String(),
	}

	// Test database connection
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := r.db.PingContext(pingCtx); err != nil {
		status = "unhealthy"
		message = fmt.Sprintf("Database connection failed: %v", err)
		details["error"] = err.Error()
	} else {
		// Get database stats
		stats := r.db.Stats()
		details["open_connections"] = stats.OpenConnections
		details["in_use"] = stats.InUse
		details["idle"] = stats.Idle
		details["wait_count"] = stats.WaitCount
		details["wait_duration"] = stats.WaitDuration.String()
		details["max_idle_closed"] = stats.MaxIdleClosed
		details["max_idle_time_closed"] = stats.MaxIdleTimeClosed
		details["max_lifetime_closed"] = stats.MaxLifetimeClosed
	}

	return portal.HealthStatus{
		Status:    status,
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
	}
}

// Close closes the repository connection and releases resources
func (r *Repository) Close() error {
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}

// BeginTx begins a transaction
func (r *Repository) BeginTx(ctx context.Context) (portal.Transaction, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, portal.NewDatabaseError("TX_BEGIN_FAILED", "failed to begin transaction", err)
	}

	return NewTransaction(r, tx), nil
}

// DB returns the underlying database connection (for internal use)
func (r *Repository) DB() *sql.DB {
	return r.db
}

// validateConnection ensures the database connection is healthy
func (r *Repository) validateConnection(ctx context.Context) error {
	if r.db == nil {
		return portal.NewDatabaseError("DB_NOT_INITIALIZED", "database connection not initialized", nil)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := r.db.PingContext(pingCtx); err != nil {
		return portal.NewDatabaseError("DB_CONNECTION_FAILED", "database connection failed", err)
	}

	return nil
}

// execQuery executes a query and returns the result
func (r *Repository) execQuery(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if err := r.validateConnection(ctx); err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, portal.NewDatabaseError("QUERY_FAILED", "database query failed", err)
	}

	return rows, nil
}

// execQueryRow executes a query that returns a single row
func (r *Repository) execQueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return r.db.QueryRowContext(ctx, query, args...)
}

// execCommand executes a command (INSERT, UPDATE, DELETE)
func (r *Repository) execCommand(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if err := r.validateConnection(ctx); err != nil {
		return nil, err
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, portal.NewDatabaseError("COMMAND_FAILED", "database command failed", err)
	}

	return result, nil
}

// isUniqueViolation checks if the error is a unique constraint violation
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	// PostgreSQL unique violation error code is 23505
	return err.Error() == "pq: duplicate key value violates unique constraint" ||
		   err.Error() == "ERROR: duplicate key value violates unique constraint"
}

// isForeignKeyViolation checks if the error is a foreign key constraint violation
func isForeignKeyViolation(err error) bool {
	if err == nil {
		return false
	}
	// PostgreSQL foreign key violation error code is 23503
	return err.Error() == "pq: insert or update on table violates foreign key constraint" ||
		   err.Error() == "ERROR: insert or update on table violates foreign key constraint"
}
