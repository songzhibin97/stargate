package memory

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/songzhibin97/stargate/pkg/portal"
)

// UserRepository implements the portal.UserRepository interface using in-memory storage
type UserRepository struct {
	repo *Repository
	tx   *Transaction
}

// NewUserRepository creates a new in-memory user repository
func NewUserRepository(repo *Repository) *UserRepository {
	return &UserRepository{
		repo: repo,
	}
}

// CreateUser creates a new user
func (ur *UserRepository) CreateUser(ctx context.Context, user *portal.User) error {
	if ur.tx != nil {
		if err := ur.tx.isActive(); err != nil {
			return err
		}
	}

	ur.repo.mu.Lock()
	defer ur.repo.mu.Unlock()

	if ur.repo.closed {
		return portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if err := ur.repo.isValidUser(user); err != nil {
		return err
	}

	// Check if user already exists
	if _, exists := ur.repo.users[user.ID]; exists {
		return portal.NewConflictError("USER_ALREADY_EXISTS", "user with this ID already exists")
	}

	// Check if email already exists
	if _, exists := ur.repo.usersByEmail[user.Email]; exists {
		return portal.NewConflictError("USER_EMAIL_EXISTS", "user with this email already exists")
	}

	// Set timestamps
	now := time.Now()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	user.UpdatedAt = now

	// Create a copy to avoid external modifications
	userCopy := *user
	ur.repo.users[user.ID] = &userCopy
	ur.repo.addUserToIndex(&userCopy)

	return nil
}

// GetUser retrieves a user by ID
func (ur *UserRepository) GetUser(ctx context.Context, userID string) (*portal.User, error) {
	if ur.tx != nil {
		if err := ur.tx.isActive(); err != nil {
			return nil, err
		}
	}

	ur.repo.mu.RLock()
	defer ur.repo.mu.RUnlock()

	if ur.repo.closed {
		return nil, portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if userID == "" {
		return nil, portal.NewValidationError("INVALID_USER_ID", "user ID cannot be empty")
	}

	user, exists := ur.repo.users[userID]
	if !exists {
		return nil, portal.NewNotFoundError("USER_NOT_FOUND", "user not found")
	}

	// Return a copy to avoid external modifications
	userCopy := *user
	return &userCopy, nil
}

// GetUserByEmail retrieves a user by email address
func (ur *UserRepository) GetUserByEmail(ctx context.Context, email string) (*portal.User, error) {
	if ur.tx != nil {
		if err := ur.tx.isActive(); err != nil {
			return nil, err
		}
	}

	ur.repo.mu.RLock()
	defer ur.repo.mu.RUnlock()

	if ur.repo.closed {
		return nil, portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if email == "" {
		return nil, portal.NewValidationError("INVALID_EMAIL", "email cannot be empty")
	}

	user, exists := ur.repo.usersByEmail[email]
	if !exists {
		return nil, portal.NewNotFoundError("USER_NOT_FOUND", "user not found")
	}

	// Return a copy to avoid external modifications
	userCopy := *user
	return &userCopy, nil
}

// UpdateUser updates an existing user
func (ur *UserRepository) UpdateUser(ctx context.Context, user *portal.User) error {
	if ur.tx != nil {
		if err := ur.tx.isActive(); err != nil {
			return err
		}
	}

	ur.repo.mu.Lock()
	defer ur.repo.mu.Unlock()

	if ur.repo.closed {
		return portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if err := ur.repo.isValidUser(user); err != nil {
		return err
	}

	// Check if user exists
	existingUser, exists := ur.repo.users[user.ID]
	if !exists {
		return portal.NewNotFoundError("USER_NOT_FOUND", "user not found")
	}

	// Check if email is being changed and if new email already exists
	if existingUser.Email != user.Email {
		if _, emailExists := ur.repo.usersByEmail[user.Email]; emailExists {
			return portal.NewConflictError("USER_EMAIL_EXISTS", "user with this email already exists")
		}
		// Remove old email from index
		ur.repo.removeUserFromIndex(existingUser)
	}

	// Update timestamps
	user.CreatedAt = existingUser.CreatedAt // Preserve original creation time
	user.UpdatedAt = time.Now()

	// Create a copy and update
	userCopy := *user
	ur.repo.users[user.ID] = &userCopy
	ur.repo.addUserToIndex(&userCopy)

	return nil
}

// DeleteUser deletes a user by ID
func (ur *UserRepository) DeleteUser(ctx context.Context, userID string) error {
	if ur.tx != nil {
		if err := ur.tx.isActive(); err != nil {
			return err
		}
	}

	ur.repo.mu.Lock()
	defer ur.repo.mu.Unlock()

	if ur.repo.closed {
		return portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if userID == "" {
		return portal.NewValidationError("INVALID_USER_ID", "user ID cannot be empty")
	}

	user, exists := ur.repo.users[userID]
	if !exists {
		return portal.NewNotFoundError("USER_NOT_FOUND", "user not found")
	}

	// Remove from indexes
	ur.repo.removeUserFromIndex(user)
	delete(ur.repo.users, userID)

	return nil
}

// ExistsUser checks if a user exists by ID
func (ur *UserRepository) ExistsUser(ctx context.Context, userID string) (bool, error) {
	if ur.tx != nil {
		if err := ur.tx.isActive(); err != nil {
			return false, err
		}
	}

	ur.repo.mu.RLock()
	defer ur.repo.mu.RUnlock()

	if ur.repo.closed {
		return false, portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if userID == "" {
		return false, portal.NewValidationError("INVALID_USER_ID", "user ID cannot be empty")
	}

	_, exists := ur.repo.users[userID]
	return exists, nil
}

// ExistsUserByEmail checks if a user exists by email
func (ur *UserRepository) ExistsUserByEmail(ctx context.Context, email string) (bool, error) {
	if ur.tx != nil {
		if err := ur.tx.isActive(); err != nil {
			return false, err
		}
	}

	ur.repo.mu.RLock()
	defer ur.repo.mu.RUnlock()

	if ur.repo.closed {
		return false, portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if email == "" {
		return false, portal.NewValidationError("INVALID_EMAIL", "email cannot be empty")
	}

	_, exists := ur.repo.usersByEmail[email]
	return exists, nil
}

// UpdateUserStatus updates the status of a user
func (ur *UserRepository) UpdateUserStatus(ctx context.Context, userID string, status portal.UserStatus) error {
	if ur.tx != nil {
		if err := ur.tx.isActive(); err != nil {
			return err
		}
	}

	ur.repo.mu.Lock()
	defer ur.repo.mu.Unlock()

	if ur.repo.closed {
		return portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if userID == "" {
		return portal.NewValidationError("INVALID_USER_ID", "user ID cannot be empty")
	}

	user, exists := ur.repo.users[userID]
	if !exists {
		return portal.NewNotFoundError("USER_NOT_FOUND", "user not found")
	}

	// Update status
	user.Status = status
	user.UpdatedAt = time.Now()

	return nil
}

// UpdateUserRole updates the role of a user
func (ur *UserRepository) UpdateUserRole(ctx context.Context, userID string, role portal.UserRole) error {
	if ur.tx != nil {
		if err := ur.tx.isActive(); err != nil {
			return err
		}
	}

	ur.repo.mu.Lock()
	defer ur.repo.mu.Unlock()

	if ur.repo.closed {
		return portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if userID == "" {
		return portal.NewValidationError("INVALID_USER_ID", "user ID cannot be empty")
	}

	user, exists := ur.repo.users[userID]
	if !exists {
		return portal.NewNotFoundError("USER_NOT_FOUND", "user not found")
	}

	// Update role
	user.Role = role
	user.UpdatedAt = time.Now()

	return nil
}

// ListUsers retrieves users based on filter criteria
func (ur *UserRepository) ListUsers(ctx context.Context, filter *portal.UserFilter) (*portal.PaginatedUsers, error) {
	if ur.tx != nil {
		if err := ur.tx.isActive(); err != nil {
			return nil, err
		}
	}

	ur.repo.mu.RLock()
	defer ur.repo.mu.RUnlock()

	if ur.repo.closed {
		return nil, portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if filter == nil {
		filter = &portal.UserFilter{}
	}

	// Set default values
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	if filter.SortBy == "" {
		filter.SortBy = "created_at"
	}
	if filter.SortOrder == "" {
		filter.SortOrder = "desc"
	}

	// Collect all users that match the filter
	var filteredUsers []*portal.User
	for _, user := range ur.repo.users {
		if ur.matchesUserFilter(user, filter) {
			userCopy := *user
			filteredUsers = append(filteredUsers, &userCopy)
		}
	}

	// Sort users
	ur.sortUsers(filteredUsers, filter.SortBy, filter.SortOrder)

	// Calculate pagination
	total := int64(len(filteredUsers))
	start := filter.Offset
	end := start + filter.Limit

	if start > len(filteredUsers) {
		start = len(filteredUsers)
	}
	if end > len(filteredUsers) {
		end = len(filteredUsers)
	}

	paginatedUsers := filteredUsers[start:end]
	hasMore := end < len(filteredUsers)

	return &portal.PaginatedUsers{
		Users:   paginatedUsers,
		Total:   total,
		Offset:  filter.Offset,
		Limit:   filter.Limit,
		HasMore: hasMore,
	}, nil
}

// CountUsers returns the total count of users matching the filter
func (ur *UserRepository) CountUsers(ctx context.Context, filter *portal.UserFilter) (int64, error) {
	if ur.tx != nil {
		if err := ur.tx.isActive(); err != nil {
			return 0, err
		}
	}

	ur.repo.mu.RLock()
	defer ur.repo.mu.RUnlock()

	if ur.repo.closed {
		return 0, portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if filter == nil {
		return int64(len(ur.repo.users)), nil
	}

	count := int64(0)
	for _, user := range ur.repo.users {
		if ur.matchesUserFilter(user, filter) {
			count++
		}
	}

	return count, nil
}

// BatchCreateUsers creates multiple users in a single operation
func (ur *UserRepository) BatchCreateUsers(ctx context.Context, users []*portal.User) error {
	if ur.tx != nil {
		if err := ur.tx.isActive(); err != nil {
			return err
		}
	}

	ur.repo.mu.Lock()
	defer ur.repo.mu.Unlock()

	if ur.repo.closed {
		return portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if len(users) == 0 {
		return nil
	}

	// Validate all users first
	for _, user := range users {
		if err := ur.repo.isValidUser(user); err != nil {
			return err
		}

		// Check if user already exists
		if _, exists := ur.repo.users[user.ID]; exists {
			return portal.NewConflictError("USER_ALREADY_EXISTS", "user with ID "+user.ID+" already exists")
		}

		// Check if email already exists
		if _, exists := ur.repo.usersByEmail[user.Email]; exists {
			return portal.NewConflictError("USER_EMAIL_EXISTS", "user with email "+user.Email+" already exists")
		}
	}

	// Create all users
	now := time.Now()
	for _, user := range users {
		// Set timestamps
		if user.CreatedAt.IsZero() {
			user.CreatedAt = now
		}
		user.UpdatedAt = now

		// Create a copy to avoid external modifications
		userCopy := *user
		ur.repo.users[user.ID] = &userCopy
		ur.repo.addUserToIndex(&userCopy)
	}

	return nil
}

// BatchUpdateUsers updates multiple users in a single operation
func (ur *UserRepository) BatchUpdateUsers(ctx context.Context, users []*portal.User) error {
	if ur.tx != nil {
		if err := ur.tx.isActive(); err != nil {
			return err
		}
	}

	ur.repo.mu.Lock()
	defer ur.repo.mu.Unlock()

	if ur.repo.closed {
		return portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if len(users) == 0 {
		return nil
	}

	// Validate all users first and check existence
	for _, user := range users {
		if err := ur.repo.isValidUser(user); err != nil {
			return err
		}

		// Check if user exists
		existingUser, exists := ur.repo.users[user.ID]
		if !exists {
			return portal.NewNotFoundError("USER_NOT_FOUND", "user with ID "+user.ID+" not found")
		}

		// Check if email is being changed and if new email already exists
		if existingUser.Email != user.Email {
			if _, emailExists := ur.repo.usersByEmail[user.Email]; emailExists {
				return portal.NewConflictError("USER_EMAIL_EXISTS", "user with email "+user.Email+" already exists")
			}
		}
	}

	// Update all users
	now := time.Now()
	for _, user := range users {
		existingUser := ur.repo.users[user.ID]

		// Remove old email from index if changed
		if existingUser.Email != user.Email {
			ur.repo.removeUserFromIndex(existingUser)
		}

		// Update timestamps
		user.CreatedAt = existingUser.CreatedAt // Preserve original creation time
		user.UpdatedAt = now

		// Create a copy and update
		userCopy := *user
		ur.repo.users[user.ID] = &userCopy
		ur.repo.addUserToIndex(&userCopy)
	}

	return nil
}

// BatchDeleteUsers deletes multiple users by IDs
func (ur *UserRepository) BatchDeleteUsers(ctx context.Context, userIDs []string) error {
	if ur.tx != nil {
		if err := ur.tx.isActive(); err != nil {
			return err
		}
	}

	ur.repo.mu.Lock()
	defer ur.repo.mu.Unlock()

	if ur.repo.closed {
		return portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if len(userIDs) == 0 {
		return nil
	}

	// Validate all user IDs first
	for _, userID := range userIDs {
		if userID == "" {
			return portal.NewValidationError("INVALID_USER_ID", "user ID cannot be empty")
		}

		if _, exists := ur.repo.users[userID]; !exists {
			return portal.NewNotFoundError("USER_NOT_FOUND", "user with ID "+userID+" not found")
		}
	}

	// Delete all users
	for _, userID := range userIDs {
		user := ur.repo.users[userID]
		ur.repo.removeUserFromIndex(user)
		delete(ur.repo.users, userID)
	}

	return nil
}

// matchesUserFilter checks if a user matches the given filter criteria
func (ur *UserRepository) matchesUserFilter(user *portal.User, filter *portal.UserFilter) bool {
	// Filter by email
	if filter.Email != "" && user.Email != filter.Email {
		return false
	}

	// Filter by name (partial match)
	if filter.Name != "" && !strings.Contains(strings.ToLower(user.Name), strings.ToLower(filter.Name)) {
		return false
	}

	// Filter by role
	if filter.Role != "" && user.Role != filter.Role {
		return false
	}

	// Filter by status
	if filter.Status != "" && user.Status != filter.Status {
		return false
	}

	// Filter by search (searches in name and email)
	if filter.Search != "" {
		searchLower := strings.ToLower(filter.Search)
		if !strings.Contains(strings.ToLower(user.Name), searchLower) &&
			!strings.Contains(strings.ToLower(user.Email), searchLower) {
			return false
		}
	}

	// Filter by creation date range
	if filter.CreatedAfter != nil && user.CreatedAt.Before(*filter.CreatedAfter) {
		return false
	}
	if filter.CreatedBefore != nil && user.CreatedAt.After(*filter.CreatedBefore) {
		return false
	}

	return true
}

// sortUsers sorts users based on the given criteria
func (ur *UserRepository) sortUsers(users []*portal.User, sortBy, sortOrder string) {
	sort.Slice(users, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "id":
			less = users[i].ID < users[j].ID
		case "email":
			less = users[i].Email < users[j].Email
		case "name":
			less = users[i].Name < users[j].Name
		case "role":
			less = string(users[i].Role) < string(users[j].Role)
		case "status":
			less = string(users[i].Status) < string(users[j].Status)
		case "updated_at":
			less = users[i].UpdatedAt.Before(users[j].UpdatedAt)
		case "created_at":
		default:
			less = users[i].CreatedAt.Before(users[j].CreatedAt)
		}

		if sortOrder == "desc" {
			return !less
		}
		return less
	})
}
