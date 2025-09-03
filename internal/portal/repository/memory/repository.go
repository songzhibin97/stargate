package memory

import (
	"context"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/pkg/portal"
)

// Repository implements the portal.Repository interface using in-memory storage
type Repository struct {
	mu           sync.RWMutex
	users        map[string]*portal.User
	applications map[string]*portal.Application
	usersByEmail map[string]*portal.User
	appsByAPIKey map[string]*portal.Application
	appsByUser   map[string][]*portal.Application
	closed       bool
}

// NewRepository creates a new in-memory repository
func NewRepository() *Repository {
	return &Repository{
		users:        make(map[string]*portal.User),
		applications: make(map[string]*portal.Application),
		usersByEmail: make(map[string]*portal.User),
		appsByAPIKey: make(map[string]*portal.Application),
		appsByUser:   make(map[string][]*portal.Application),
	}
}

// Health returns the health status of the repository
func (r *Repository) Health(ctx context.Context) portal.HealthStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	status := "healthy"
	message := "In-memory repository is operational"
	details := map[string]interface{}{
		"users_count":        len(r.users),
		"applications_count": len(r.applications),
		"closed":            r.closed,
	}

	if r.closed {
		status = "unhealthy"
		message = "Repository is closed"
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
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}

	// Clear all data
	r.users = nil
	r.applications = nil
	r.usersByEmail = nil
	r.appsByAPIKey = nil
	r.appsByUser = nil
	r.closed = true

	return nil
}

// BeginTx begins a transaction
func (r *Repository) BeginTx(ctx context.Context) (portal.Transaction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed {
		return nil, portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	return NewTransaction(r), nil
}

// isValidUser validates user data
func (r *Repository) isValidUser(user *portal.User) error {
	if user == nil {
		return portal.NewValidationError("INVALID_USER", "user cannot be nil")
	}
	if user.ID == "" {
		return portal.NewValidationError("INVALID_USER_ID", "user ID cannot be empty")
	}
	if user.Email == "" {
		return portal.NewValidationError("INVALID_USER_EMAIL", "user email cannot be empty")
	}
	if user.Name == "" {
		return portal.NewValidationError("INVALID_USER_NAME", "user name cannot be empty")
	}
	if user.Role == "" {
		return portal.NewValidationError("INVALID_USER_ROLE", "user role cannot be empty")
	}
	if user.Status == "" {
		return portal.NewValidationError("INVALID_USER_STATUS", "user status cannot be empty")
	}
	return nil
}

// isValidApplication validates application data
func (r *Repository) isValidApplication(app *portal.Application) error {
	if app == nil {
		return portal.NewValidationError("INVALID_APPLICATION", "application cannot be nil")
	}
	if app.ID == "" {
		return portal.NewValidationError("INVALID_APPLICATION_ID", "application ID cannot be empty")
	}
	if app.Name == "" {
		return portal.NewValidationError("INVALID_APPLICATION_NAME", "application name cannot be empty")
	}
	if app.UserID == "" {
		return portal.NewValidationError("INVALID_APPLICATION_USER_ID", "application user ID cannot be empty")
	}
	if app.APIKey == "" {
		return portal.NewValidationError("INVALID_APPLICATION_API_KEY", "application API key cannot be empty")
	}
	if app.Status == "" {
		return portal.NewValidationError("INVALID_APPLICATION_STATUS", "application status cannot be empty")
	}
	return nil
}

// addUserToIndex adds user to internal indexes
func (r *Repository) addUserToIndex(user *portal.User) {
	r.usersByEmail[user.Email] = user
}

// removeUserFromIndex removes user from internal indexes
func (r *Repository) removeUserFromIndex(user *portal.User) {
	delete(r.usersByEmail, user.Email)
}

// addApplicationToIndex adds application to internal indexes
func (r *Repository) addApplicationToIndex(app *portal.Application) {
	r.appsByAPIKey[app.APIKey] = app
	r.appsByUser[app.UserID] = append(r.appsByUser[app.UserID], app)
}

// removeApplicationFromIndex removes application from internal indexes
func (r *Repository) removeApplicationFromIndex(app *portal.Application) {
	delete(r.appsByAPIKey, app.APIKey)
	
	// Remove from user's applications
	if apps, exists := r.appsByUser[app.UserID]; exists {
		for i, userApp := range apps {
			if userApp.ID == app.ID {
				r.appsByUser[app.UserID] = append(apps[:i], apps[i+1:]...)
				break
			}
		}
		// Clean up empty slice
		if len(r.appsByUser[app.UserID]) == 0 {
			delete(r.appsByUser, app.UserID)
		}
	}
}
