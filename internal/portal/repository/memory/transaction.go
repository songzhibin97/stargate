package memory

import (
	"context"
	"sync"

	"github.com/songzhibin97/stargate/pkg/portal"
)

// Transaction implements the portal.Transaction interface for in-memory storage
type Transaction struct {
	repo      *Repository
	userRepo  *UserRepository
	appRepo   *ApplicationRepository
	committed bool
	rolledBack bool
	mu        sync.Mutex
}

// NewTransaction creates a new transaction
func NewTransaction(repo *Repository) *Transaction {
	tx := &Transaction{
		repo: repo,
	}
	
	// Create repository instances that share the same transaction context
	tx.userRepo = &UserRepository{
		repo: repo,
		tx:   tx,
	}
	tx.appRepo = &ApplicationRepository{
		repo: repo,
		tx:   tx,
	}
	
	return tx
}

// Commit commits the transaction
func (tx *Transaction) Commit(ctx context.Context) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.committed {
		return portal.NewDatabaseError("TX_ALREADY_COMMITTED", "transaction already committed", nil)
	}
	if tx.rolledBack {
		return portal.NewDatabaseError("TX_ALREADY_ROLLED_BACK", "transaction already rolled back", nil)
	}

	// For in-memory implementation, operations are applied immediately
	// so commit is just a state change
	tx.committed = true
	return nil
}

// Rollback rolls back the transaction
func (tx *Transaction) Rollback(ctx context.Context) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.committed {
		return portal.NewDatabaseError("TX_ALREADY_COMMITTED", "transaction already committed", nil)
	}
	if tx.rolledBack {
		return portal.NewDatabaseError("TX_ALREADY_ROLLED_BACK", "transaction already rolled back", nil)
	}

	// For in-memory implementation, we can't really rollback changes
	// In a real database implementation, this would undo all changes
	tx.rolledBack = true
	return nil
}

// UserRepository returns a user repository within this transaction
func (tx *Transaction) UserRepository() portal.UserRepository {
	return tx.userRepo
}

// ApplicationRepository returns an application repository within this transaction
func (tx *Transaction) ApplicationRepository() portal.ApplicationRepository {
	return tx.appRepo
}

// isActive checks if the transaction is still active
func (tx *Transaction) isActive() error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.committed {
		return portal.NewDatabaseError("TX_COMMITTED", "transaction is committed", nil)
	}
	if tx.rolledBack {
		return portal.NewDatabaseError("TX_ROLLED_BACK", "transaction is rolled back", nil)
	}
	return nil
}
