package portal

import (
	"context"
	"time"
)

// Repository defines the base interface for all repositories
type Repository interface {
	// Health returns the health status of the repository
	Health(ctx context.Context) HealthStatus
	
	// Close closes the repository connection and releases resources
	Close() error
	
	// BeginTx begins a transaction
	BeginTx(ctx context.Context) (Transaction, error)
}

// Transaction defines the interface for database transactions
type Transaction interface {
	// Commit commits the transaction
	Commit(ctx context.Context) error
	
	// Rollback rolls back the transaction
	Rollback(ctx context.Context) error
	
	// UserRepository returns a user repository within this transaction
	UserRepository() UserRepository
	
	// ApplicationRepository returns an application repository within this transaction
	ApplicationRepository() ApplicationRepository
}

// HealthStatus represents the health status of a repository
type HealthStatus struct {
	Status    string                 `json:"status"`
	Message   string                 `json:"message,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// UserRepository defines the interface for user data operations
type UserRepository interface {
	// CreateUser creates a new user
	CreateUser(ctx context.Context, user *User) error
	
	// GetUser retrieves a user by ID
	GetUser(ctx context.Context, userID string) (*User, error)
	
	// GetUserByEmail retrieves a user by email address
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	
	// UpdateUser updates an existing user
	UpdateUser(ctx context.Context, user *User) error
	
	// DeleteUser deletes a user by ID
	DeleteUser(ctx context.Context, userID string) error
	
	// ListUsers retrieves users based on filter criteria
	ListUsers(ctx context.Context, filter *UserFilter) (*PaginatedUsers, error)
	
	// CountUsers returns the total count of users matching the filter
	CountUsers(ctx context.Context, filter *UserFilter) (int64, error)
	
	// ExistsUser checks if a user exists by ID
	ExistsUser(ctx context.Context, userID string) (bool, error)
	
	// ExistsUserByEmail checks if a user exists by email
	ExistsUserByEmail(ctx context.Context, email string) (bool, error)
	
	// UpdateUserStatus updates the status of a user
	UpdateUserStatus(ctx context.Context, userID string, status UserStatus) error
	
	// UpdateUserRole updates the role of a user
	UpdateUserRole(ctx context.Context, userID string, role UserRole) error
	
	// BatchCreateUsers creates multiple users in a single operation
	BatchCreateUsers(ctx context.Context, users []*User) error
	
	// BatchUpdateUsers updates multiple users in a single operation
	BatchUpdateUsers(ctx context.Context, users []*User) error
	
	// BatchDeleteUsers deletes multiple users by IDs
	BatchDeleteUsers(ctx context.Context, userIDs []string) error
}

// ApplicationRepository defines the interface for application data operations
type ApplicationRepository interface {
	// CreateApplication creates a new application
	CreateApplication(ctx context.Context, app *Application) error
	
	// GetApplication retrieves an application by ID
	GetApplication(ctx context.Context, appID string) (*Application, error)
	
	// GetApplicationByAPIKey retrieves an application by API key
	GetApplicationByAPIKey(ctx context.Context, apiKey string) (*Application, error)
	
	// GetApplicationsByUser retrieves all applications for a specific user
	GetApplicationsByUser(ctx context.Context, userID string) ([]*Application, error)
	
	// UpdateApplication updates an existing application
	UpdateApplication(ctx context.Context, app *Application) error
	
	// DeleteApplication deletes an application by ID
	DeleteApplication(ctx context.Context, appID string) error
	
	// ListApplications retrieves applications based on filter criteria
	ListApplications(ctx context.Context, filter *ApplicationFilter) (*PaginatedApplications, error)
	
	// CountApplications returns the total count of applications matching the filter
	CountApplications(ctx context.Context, filter *ApplicationFilter) (int64, error)
	
	// ExistsApplication checks if an application exists by ID
	ExistsApplication(ctx context.Context, appID string) (bool, error)
	
	// ExistsApplicationByAPIKey checks if an application exists by API key
	ExistsApplicationByAPIKey(ctx context.Context, apiKey string) (bool, error)
	
	// UpdateApplicationStatus updates the status of an application
	UpdateApplicationStatus(ctx context.Context, appID string, status ApplicationStatus) error
	
	// UpdateApplicationRateLimit updates the rate limit of an application
	UpdateApplicationRateLimit(ctx context.Context, appID string, rateLimit int64) error
	
	// RegenerateAPIKey generates a new API key for an application
	RegenerateAPIKey(ctx context.Context, appID string) (string, error)
	
	// RegenerateAPISecret generates a new API secret for an application
	RegenerateAPISecret(ctx context.Context, appID string) (string, error)
	
	// BatchCreateApplications creates multiple applications in a single operation
	BatchCreateApplications(ctx context.Context, apps []*Application) error
	
	// BatchUpdateApplications updates multiple applications in a single operation
	BatchUpdateApplications(ctx context.Context, apps []*Application) error
	
	// BatchDeleteApplications deletes multiple applications by IDs
	BatchDeleteApplications(ctx context.Context, appIDs []string) error
	
	// CountApplicationsByUser returns the count of applications for a specific user
	CountApplicationsByUser(ctx context.Context, userID string) (int64, error)
}
