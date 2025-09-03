package postgres

import (
	"context"
	"database/sql"
	"sync"

	"github.com/songzhibin97/stargate/pkg/portal"
)

// Transaction implements the portal.Transaction interface for PostgreSQL
type Transaction struct {
	repo       *Repository
	tx         *sql.Tx
	userRepo   *UserRepository
	appRepo    *ApplicationRepository
	committed  bool
	rolledBack bool
	mu         sync.Mutex
}

// NewTransaction creates a new PostgreSQL transaction
func NewTransaction(repo *Repository, tx *sql.Tx) *Transaction {
	transaction := &Transaction{
		repo: repo,
		tx:   tx,
	}

	// Create repository instances that share the same transaction context
	transaction.userRepo = &UserRepository{
		repo: repo,
		tx:   transaction,
	}
	transaction.appRepo = &ApplicationRepository{
		repo: repo,
		tx:   transaction,
	}

	return transaction
}

// Commit commits the transaction
func (t *Transaction) Commit(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.committed {
		return portal.NewDatabaseError("TX_ALREADY_COMMITTED", "transaction already committed", nil)
	}
	if t.rolledBack {
		return portal.NewDatabaseError("TX_ALREADY_ROLLED_BACK", "transaction already rolled back", nil)
	}

	if err := t.tx.Commit(); err != nil {
		return portal.NewDatabaseError("TX_COMMIT_FAILED", "failed to commit transaction", err)
	}

	t.committed = true
	return nil
}

// Rollback rolls back the transaction
func (t *Transaction) Rollback(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.committed {
		return portal.NewDatabaseError("TX_ALREADY_COMMITTED", "transaction already committed", nil)
	}
	if t.rolledBack {
		return portal.NewDatabaseError("TX_ALREADY_ROLLED_BACK", "transaction already rolled back", nil)
	}

	if err := t.tx.Rollback(); err != nil {
		return portal.NewDatabaseError("TX_ROLLBACK_FAILED", "failed to rollback transaction", err)
	}

	t.rolledBack = true
	return nil
}

// UserRepository returns a user repository within this transaction
func (t *Transaction) UserRepository() portal.UserRepository {
	return t.userRepo
}

// ApplicationRepository returns an application repository within this transaction
func (t *Transaction) ApplicationRepository() portal.ApplicationRepository {
	return t.appRepo
}

// isActive checks if the transaction is still active
func (t *Transaction) isActive() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.committed {
		return portal.NewDatabaseError("TX_COMMITTED", "transaction is committed", nil)
	}
	if t.rolledBack {
		return portal.NewDatabaseError("TX_ROLLED_BACK", "transaction is rolled back", nil)
	}
	return nil
}

// execQuery executes a query within the transaction
func (t *Transaction) execQuery(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if err := t.isActive(); err != nil {
		return nil, err
	}

	rows, err := t.tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, portal.NewDatabaseError("QUERY_FAILED", "database query failed", err)
	}

	return rows, nil
}

// execQueryRow executes a query that returns a single row within the transaction
func (t *Transaction) execQueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return t.tx.QueryRowContext(ctx, query, args...)
}

// execCommand executes a command (INSERT, UPDATE, DELETE) within the transaction
func (t *Transaction) execCommand(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if err := t.isActive(); err != nil {
		return nil, err
	}

	result, err := t.tx.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, portal.NewDatabaseError("COMMAND_FAILED", "database command failed", err)
	}

	return result, nil
}
