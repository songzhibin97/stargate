package portal

import (
	"time"
)

// User represents a developer portal user
type User struct {
	ID        string    `json:"id" db:"id"`
	Email     string    `json:"email" db:"email"`
	Name      string    `json:"name" db:"name"`
	Password  string    `json:"-" db:"password"` // Password hash, never included in JSON responses
	Role      UserRole  `json:"role" db:"role"`
	Status    UserStatus `json:"status" db:"status"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// UserRole represents the role of a user
type UserRole string

const (
	UserRoleAdmin     UserRole = "admin"
	UserRoleDeveloper UserRole = "developer"
	UserRoleViewer    UserRole = "viewer"
)

// UserStatus represents the status of a user
type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusInactive UserStatus = "inactive"
	UserStatusSuspended UserStatus = "suspended"
)

// Application represents a developer application
type Application struct {
	ID          string            `json:"id" db:"id"`
	Name        string            `json:"name" db:"name"`
	Description string            `json:"description" db:"description"`
	UserID      string            `json:"user_id" db:"user_id"`
	APIKey      string            `json:"api_key" db:"api_key"`
	APISecret   string            `json:"api_secret" db:"api_secret"`
	Status      ApplicationStatus `json:"status" db:"status"`
	RateLimit   int64             `json:"rate_limit" db:"rate_limit"`
	CreatedAt   time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at" db:"updated_at"`
}

// ApplicationStatus represents the status of an application
type ApplicationStatus string

const (
	ApplicationStatusActive   ApplicationStatus = "active"
	ApplicationStatusInactive ApplicationStatus = "inactive"
	ApplicationStatusSuspended ApplicationStatus = "suspended"
)

// UserFilter represents filter criteria for user queries
type UserFilter struct {
	// Pagination
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
	
	// Sorting
	SortBy    string `json:"sort_by"`
	SortOrder string `json:"sort_order"` // "asc" or "desc"
	
	// Filtering
	Email  string     `json:"email,omitempty"`
	Name   string     `json:"name,omitempty"`
	Role   UserRole   `json:"role,omitempty"`
	Status UserStatus `json:"status,omitempty"`
	
	// Search
	Search string `json:"search,omitempty"`
	
	// Date range
	CreatedAfter  *time.Time `json:"created_after,omitempty"`
	CreatedBefore *time.Time `json:"created_before,omitempty"`
}

// ApplicationFilter represents filter criteria for application queries
type ApplicationFilter struct {
	// Pagination
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
	
	// Sorting
	SortBy    string `json:"sort_by"`
	SortOrder string `json:"sort_order"` // "asc" or "desc"
	
	// Filtering
	UserID string            `json:"user_id,omitempty"`
	Name   string            `json:"name,omitempty"`
	Status ApplicationStatus `json:"status,omitempty"`
	
	// Search
	Search string `json:"search,omitempty"`
	
	// Date range
	CreatedAfter  *time.Time `json:"created_after,omitempty"`
	CreatedBefore *time.Time `json:"created_before,omitempty"`
}

// PaginatedUsers represents a paginated list of users
type PaginatedUsers struct {
	Users      []*User `json:"users"`
	Total      int64   `json:"total"`
	Offset     int     `json:"offset"`
	Limit      int     `json:"limit"`
	HasMore    bool    `json:"has_more"`
}

// PaginatedApplications represents a paginated list of applications
type PaginatedApplications struct {
	Applications []*Application `json:"applications"`
	Total        int64          `json:"total"`
	Offset       int            `json:"offset"`
	Limit        int            `json:"limit"`
	HasMore      bool           `json:"has_more"`
}
