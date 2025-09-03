// Package portal provides interfaces and types for developer portal data operations.
//
// This package defines the data access layer for the developer portal, including
// user management and application management functionality. It provides a clean
// abstraction over the underlying data storage implementation.
//
// # Architecture
//
// The portal package follows the Repository pattern to abstract data access:
//
//   - Repository: Base interface for all repositories with health check and transaction support
//   - UserRepository: Interface for user-related data operations
//   - ApplicationRepository: Interface for application-related data operations
//   - Transaction: Interface for transactional operations across multiple repositories
//
// # Error Handling
//
// The package defines structured error types for consistent error handling:
//
//   - PortalError: Structured error with type, code, message, and optional details
//   - Error types: NotFound, Conflict, Validation, Permission, Database, Internal
//   - Helper functions: IsNotFoundError, IsConflictError, etc.
//
// # Data Models
//
// Core data models include:
//
//   - User: Represents a developer portal user with role-based access
//   - Application: Represents a developer application with API credentials
//   - UserFilter/ApplicationFilter: Filter criteria for querying data
//   - PaginatedUsers/PaginatedApplications: Paginated result sets
//
// # Usage Examples
//
// ## Basic User Operations
//
//	// Create a new user
//	user := &portal.User{
//		ID:     "user-123",
//		Email:  "developer@example.com",
//		Name:   "John Developer",
//		Role:   portal.UserRoleDeveloper,
//		Status: portal.UserStatusActive,
//	}
//	err := userRepo.CreateUser(ctx, user)
//	if err != nil {
//		if portal.IsConflictError(err) {
//			// Handle user already exists
//		}
//		return err
//	}
//
//	// Get user by email
//	user, err := userRepo.GetUserByEmail(ctx, "developer@example.com")
//	if err != nil {
//		if portal.IsNotFoundError(err) {
//			// Handle user not found
//		}
//		return err
//	}
//
//	// List users with filtering
//	filter := &portal.UserFilter{
//		Role:      portal.UserRoleDeveloper,
//		Status:    portal.UserStatusActive,
//		Limit:     10,
//		Offset:    0,
//		SortBy:    "created_at",
//		SortOrder: "desc",
//	}
//	result, err := userRepo.ListUsers(ctx, filter)
//	if err != nil {
//		return err
//	}
//	for _, user := range result.Users {
//		fmt.Printf("User: %s (%s)\n", user.Name, user.Email)
//	}
//
// ## Basic Application Operations
//
//	// Create a new application
//	app := &portal.Application{
//		ID:          "app-456",
//		Name:        "My API Client",
//		Description: "Client application for API access",
//		UserID:      "user-123",
//		APIKey:      "ak_1234567890abcdef",
//		APISecret:   "as_abcdef1234567890",
//		Status:      portal.ApplicationStatusActive,
//		RateLimit:   1000,
//	}
//	err := appRepo.CreateApplication(ctx, app)
//	if err != nil {
//		return err
//	}
//
//	// Get applications for a user
//	apps, err := appRepo.GetApplicationsByUser(ctx, "user-123")
//	if err != nil {
//		return err
//	}
//	for _, app := range apps {
//		fmt.Printf("App: %s (Rate Limit: %d)\n", app.Name, app.RateLimit)
//	}
//
//	// Regenerate API credentials
//	newAPIKey, err := appRepo.RegenerateAPIKey(ctx, "app-456")
//	if err != nil {
//		return err
//	}
//	fmt.Printf("New API Key: %s\n", newAPIKey)
//
// ## Transaction Usage
//
//	// Perform operations within a transaction
//	tx, err := repo.BeginTx(ctx)
//	if err != nil {
//		return err
//	}
//	defer func() {
//		if err != nil {
//			tx.Rollback(ctx)
//		}
//	}()
//
//	// Create user and application atomically
//	userRepo := tx.UserRepository()
//	appRepo := tx.ApplicationRepository()
//
//	err = userRepo.CreateUser(ctx, user)
//	if err != nil {
//		return err
//	}
//
//	err = appRepo.CreateApplication(ctx, app)
//	if err != nil {
//		return err
//	}
//
//	err = tx.Commit(ctx)
//	if err != nil {
//		return err
//	}
//
// ## Error Handling Best Practices
//
//	err := userRepo.GetUser(ctx, "nonexistent-user")
//	if err != nil {
//		switch {
//		case portal.IsNotFoundError(err):
//			// Handle not found - maybe return 404 to client
//			return fmt.Errorf("user not found")
//		case portal.IsValidationError(err):
//			// Handle validation error - return 400 to client
//			return fmt.Errorf("invalid input: %w", err)
//		case portal.IsDatabaseError(err):
//			// Handle database error - maybe retry or return 500
//			log.Error("database error", "error", err)
//			return fmt.Errorf("internal server error")
//		default:
//			// Handle unexpected error
//			log.Error("unexpected error", "error", err)
//			return fmt.Errorf("internal server error")
//		}
//	}
//
// # Implementation Notes
//
// Implementations of these interfaces should:
//
//   - Be thread-safe for concurrent access
//   - Support context cancellation and timeouts
//   - Provide proper error handling with structured error types
//   - Support database transactions where applicable
//   - Implement proper pagination for list operations
//   - Validate input data before persistence
//   - Handle database constraints and conflicts gracefully
//   - Provide health check functionality
//   - Support batch operations for performance
//
// # Database Schema Considerations
//
// When implementing these interfaces, consider:
//
//   - Proper indexing on frequently queried fields (email, user_id, api_key)
//   - Unique constraints on email addresses and API keys
//   - Foreign key relationships between users and applications
//   - Soft delete support for audit trails
//   - Timestamp fields for created_at and updated_at
//   - Rate limiting fields for application throttling
//   - Status fields for enabling/disabling entities
//
package portal
