package memory

import (
	"context"
	"testing"

	"github.com/songzhibin97/stargate/pkg/portal"
)

func TestUserRepository_CreateUser(t *testing.T) {
	repo := NewRepository()
	userRepo := NewUserRepository(repo)
	ctx := context.Background()

	user := createTestUser("user1", "test@example.com")

	// Test successful creation
	err := userRepo.CreateUser(ctx, user)
	if err != nil {
		t.Errorf("CreateUser() returned error: %v", err)
	}

	// Verify user was stored
	storedUser, exists := repo.users[user.ID]
	if !exists {
		t.Error("User not stored in repository")
	}
	if storedUser.Email != user.Email {
		t.Errorf("Expected email %s, got %s", user.Email, storedUser.Email)
	}

	// Test duplicate ID
	duplicateUser := createTestUser("user1", "different@example.com")
	err = userRepo.CreateUser(ctx, duplicateUser)
	if err == nil {
		t.Error("Expected error for duplicate user ID")
	}
	if !portal.IsConflictError(err) {
		t.Errorf("Expected conflict error, got: %v", err)
	}

	// Test duplicate email
	duplicateEmailUser := createTestUser("user2", "test@example.com")
	err = userRepo.CreateUser(ctx, duplicateEmailUser)
	if err == nil {
		t.Error("Expected error for duplicate email")
	}
	if !portal.IsConflictError(err) {
		t.Errorf("Expected conflict error, got: %v", err)
	}

	// Test closed repository
	repo.Close()
	err = userRepo.CreateUser(ctx, createTestUser("user3", "test3@example.com"))
	if err == nil {
		t.Error("Expected error for closed repository")
	}
	if !portal.IsDatabaseError(err) {
		t.Errorf("Expected database error, got: %v", err)
	}
}

func TestUserRepository_GetUser(t *testing.T) {
	repo := NewRepository()
	userRepo := NewUserRepository(repo)
	ctx := context.Background()

	user := createTestUser("user1", "test@example.com")
	userRepo.CreateUser(ctx, user)

	// Test successful retrieval
	retrievedUser, err := userRepo.GetUser(ctx, "user1")
	if err != nil {
		t.Errorf("GetUser() returned error: %v", err)
	}
	if retrievedUser.ID != user.ID {
		t.Errorf("Expected ID %s, got %s", user.ID, retrievedUser.ID)
	}
	if retrievedUser.Email != user.Email {
		t.Errorf("Expected email %s, got %s", user.Email, retrievedUser.Email)
	}

	// Test non-existent user
	_, err = userRepo.GetUser(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent user")
	}
	if !portal.IsNotFoundError(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}

	// Test empty ID
	_, err = userRepo.GetUser(ctx, "")
	if err == nil {
		t.Error("Expected error for empty user ID")
	}
	if !portal.IsValidationError(err) {
		t.Errorf("Expected validation error, got: %v", err)
	}

	// Test closed repository
	repo.Close()
	_, err = userRepo.GetUser(ctx, "user1")
	if err == nil {
		t.Error("Expected error for closed repository")
	}
	if !portal.IsDatabaseError(err) {
		t.Errorf("Expected database error, got: %v", err)
	}
}

func TestUserRepository_GetUserByEmail(t *testing.T) {
	repo := NewRepository()
	userRepo := NewUserRepository(repo)
	ctx := context.Background()

	user := createTestUser("user1", "test@example.com")
	userRepo.CreateUser(ctx, user)

	// Test successful retrieval
	retrievedUser, err := userRepo.GetUserByEmail(ctx, "test@example.com")
	if err != nil {
		t.Errorf("GetUserByEmail() returned error: %v", err)
	}
	if retrievedUser.ID != user.ID {
		t.Errorf("Expected ID %s, got %s", user.ID, retrievedUser.ID)
	}

	// Test non-existent email
	_, err = userRepo.GetUserByEmail(ctx, "nonexistent@example.com")
	if err == nil {
		t.Error("Expected error for non-existent email")
	}
	if !portal.IsNotFoundError(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}

	// Test empty email
	_, err = userRepo.GetUserByEmail(ctx, "")
	if err == nil {
		t.Error("Expected error for empty email")
	}
	if !portal.IsValidationError(err) {
		t.Errorf("Expected validation error, got: %v", err)
	}
}

func TestUserRepository_UpdateUser(t *testing.T) {
	repo := NewRepository()
	userRepo := NewUserRepository(repo)
	ctx := context.Background()

	user := createTestUser("user1", "test@example.com")
	userRepo.CreateUser(ctx, user)

	// Test successful update
	user.Name = "Updated Name"
	user.Role = portal.UserRoleAdmin
	err := userRepo.UpdateUser(ctx, user)
	if err != nil {
		t.Errorf("UpdateUser() returned error: %v", err)
	}

	// Verify update
	updatedUser, _ := userRepo.GetUser(ctx, "user1")
	if updatedUser.Name != "Updated Name" {
		t.Errorf("Expected name 'Updated Name', got '%s'", updatedUser.Name)
	}
	if updatedUser.Role != portal.UserRoleAdmin {
		t.Errorf("Expected role %s, got %s", portal.UserRoleAdmin, updatedUser.Role)
	}

	// Test email change
	user.Email = "newemail@example.com"
	err = userRepo.UpdateUser(ctx, user)
	if err != nil {
		t.Errorf("UpdateUser() with email change returned error: %v", err)
	}

	// Verify email index was updated
	_, err = userRepo.GetUserByEmail(ctx, "test@example.com")
	if err == nil {
		t.Error("Old email should not exist in index")
	}
	_, err = userRepo.GetUserByEmail(ctx, "newemail@example.com")
	if err != nil {
		t.Error("New email should exist in index")
	}

	// Test non-existent user
	nonExistentUser := createTestUser("nonexistent", "nonexistent@example.com")
	err = userRepo.UpdateUser(ctx, nonExistentUser)
	if err == nil {
		t.Error("Expected error for non-existent user")
	}
	if !portal.IsNotFoundError(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}

	// Test email conflict
	user2 := createTestUser("user2", "user2@example.com")
	userRepo.CreateUser(ctx, user2)
	
	user2.Email = "newemail@example.com" // Same as user1's current email
	err = userRepo.UpdateUser(ctx, user2)
	if err == nil {
		t.Error("Expected error for email conflict")
	}
	if !portal.IsConflictError(err) {
		t.Errorf("Expected conflict error, got: %v", err)
	}
}

func TestUserRepository_DeleteUser(t *testing.T) {
	repo := NewRepository()
	userRepo := NewUserRepository(repo)
	ctx := context.Background()

	user := createTestUser("user1", "test@example.com")
	userRepo.CreateUser(ctx, user)

	// Test successful deletion
	err := userRepo.DeleteUser(ctx, "user1")
	if err != nil {
		t.Errorf("DeleteUser() returned error: %v", err)
	}

	// Verify user was deleted
	_, err = userRepo.GetUser(ctx, "user1")
	if err == nil {
		t.Error("User should be deleted")
	}
	if !portal.IsNotFoundError(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}

	// Verify email index was cleaned up
	_, err = userRepo.GetUserByEmail(ctx, "test@example.com")
	if err == nil {
		t.Error("Email should be removed from index")
	}

	// Test non-existent user
	err = userRepo.DeleteUser(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent user")
	}
	if !portal.IsNotFoundError(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}

	// Test empty ID
	err = userRepo.DeleteUser(ctx, "")
	if err == nil {
		t.Error("Expected error for empty user ID")
	}
	if !portal.IsValidationError(err) {
		t.Errorf("Expected validation error, got: %v", err)
	}
}

func TestUserRepository_ExistsUser(t *testing.T) {
	repo := NewRepository()
	userRepo := NewUserRepository(repo)
	ctx := context.Background()

	user := createTestUser("user1", "test@example.com")
	userRepo.CreateUser(ctx, user)

	// Test existing user
	exists, err := userRepo.ExistsUser(ctx, "user1")
	if err != nil {
		t.Errorf("ExistsUser() returned error: %v", err)
	}
	if !exists {
		t.Error("User should exist")
	}

	// Test non-existent user
	exists, err = userRepo.ExistsUser(ctx, "nonexistent")
	if err != nil {
		t.Errorf("ExistsUser() returned error: %v", err)
	}
	if exists {
		t.Error("User should not exist")
	}

	// Test empty ID
	_, err = userRepo.ExistsUser(ctx, "")
	if err == nil {
		t.Error("Expected error for empty user ID")
	}
	if !portal.IsValidationError(err) {
		t.Errorf("Expected validation error, got: %v", err)
	}
}

func TestUserRepository_UpdateUserStatus(t *testing.T) {
	repo := NewRepository()
	userRepo := NewUserRepository(repo)
	ctx := context.Background()

	user := createTestUser("user1", "test@example.com")
	userRepo.CreateUser(ctx, user)

	// Test successful status update
	err := userRepo.UpdateUserStatus(ctx, "user1", portal.UserStatusSuspended)
	if err != nil {
		t.Errorf("UpdateUserStatus() returned error: %v", err)
	}

	// Verify status was updated
	updatedUser, _ := userRepo.GetUser(ctx, "user1")
	if updatedUser.Status != portal.UserStatusSuspended {
		t.Errorf("Expected status %s, got %s", portal.UserStatusSuspended, updatedUser.Status)
	}

	// Test non-existent user
	err = userRepo.UpdateUserStatus(ctx, "nonexistent", portal.UserStatusActive)
	if err == nil {
		t.Error("Expected error for non-existent user")
	}
	if !portal.IsNotFoundError(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}

	// Test empty ID
	err = userRepo.UpdateUserStatus(ctx, "", portal.UserStatusActive)
	if err == nil {
		t.Error("Expected error for empty user ID")
	}
	if !portal.IsValidationError(err) {
		t.Errorf("Expected validation error, got: %v", err)
	}
}

func TestUserRepository_UpdateUserRole(t *testing.T) {
	repo := NewRepository()
	userRepo := NewUserRepository(repo)
	ctx := context.Background()

	user := createTestUser("user1", "test@example.com")
	userRepo.CreateUser(ctx, user)

	// Test successful role update
	err := userRepo.UpdateUserRole(ctx, "user1", portal.UserRoleAdmin)
	if err != nil {
		t.Errorf("UpdateUserRole() returned error: %v", err)
	}

	// Verify role was updated
	updatedUser, _ := userRepo.GetUser(ctx, "user1")
	if updatedUser.Role != portal.UserRoleAdmin {
		t.Errorf("Expected role %s, got %s", portal.UserRoleAdmin, updatedUser.Role)
	}

	// Test non-existent user
	err = userRepo.UpdateUserRole(ctx, "nonexistent", portal.UserRoleViewer)
	if err == nil {
		t.Error("Expected error for non-existent user")
	}
	if !portal.IsNotFoundError(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}

	// Test empty ID
	err = userRepo.UpdateUserRole(ctx, "", portal.UserRoleViewer)
	if err == nil {
		t.Error("Expected error for empty user ID")
	}
	if !portal.IsValidationError(err) {
		t.Errorf("Expected validation error, got: %v", err)
	}
}

func TestUserRepository_ListUsers(t *testing.T) {
	repo := NewRepository()
	userRepo := NewUserRepository(repo)
	ctx := context.Background()

	// Create test users
	users := []*portal.User{
		createTestUser("user1", "user1@example.com"),
		createTestUser("user2", "user2@example.com"),
		createTestUser("user3", "user3@example.com"),
	}
	users[1].Role = portal.UserRoleAdmin
	users[2].Status = portal.UserStatusSuspended

	for _, user := range users {
		userRepo.CreateUser(ctx, user)
	}

	// Test list all users
	result, err := userRepo.ListUsers(ctx, nil)
	if err != nil {
		t.Errorf("ListUsers() returned error: %v", err)
	}
	if len(result.Users) != 3 {
		t.Errorf("Expected 3 users, got %d", len(result.Users))
	}
	if result.Total != 3 {
		t.Errorf("Expected total 3, got %d", result.Total)
	}

	// Test filter by role
	filter := &portal.UserFilter{Role: portal.UserRoleAdmin}
	result, err = userRepo.ListUsers(ctx, filter)
	if err != nil {
		t.Errorf("ListUsers() with role filter returned error: %v", err)
	}
	if len(result.Users) != 1 {
		t.Errorf("Expected 1 admin user, got %d", len(result.Users))
	}
	if result.Users[0].Role != portal.UserRoleAdmin {
		t.Error("Filtered user should be admin")
	}

	// Test filter by status
	filter = &portal.UserFilter{Status: portal.UserStatusSuspended}
	result, err = userRepo.ListUsers(ctx, filter)
	if err != nil {
		t.Errorf("ListUsers() with status filter returned error: %v", err)
	}
	if len(result.Users) != 1 {
		t.Errorf("Expected 1 suspended user, got %d", len(result.Users))
	}

	// Test pagination
	filter = &portal.UserFilter{Limit: 2, Offset: 0}
	result, err = userRepo.ListUsers(ctx, filter)
	if err != nil {
		t.Errorf("ListUsers() with pagination returned error: %v", err)
	}
	if len(result.Users) != 2 {
		t.Errorf("Expected 2 users in first page, got %d", len(result.Users))
	}
	if !result.HasMore {
		t.Error("Should have more users")
	}

	// Test search
	filter = &portal.UserFilter{Search: "user1"}
	result, err = userRepo.ListUsers(ctx, filter)
	if err != nil {
		t.Errorf("ListUsers() with search returned error: %v", err)
	}
	if len(result.Users) != 1 {
		t.Errorf("Expected 1 user matching search, got %d", len(result.Users))
	}
}

func TestUserRepository_CountUsers(t *testing.T) {
	repo := NewRepository()
	userRepo := NewUserRepository(repo)
	ctx := context.Background()

	// Create test users
	users := []*portal.User{
		createTestUser("user1", "user1@example.com"),
		createTestUser("user2", "user2@example.com"),
		createTestUser("user3", "user3@example.com"),
	}
	users[1].Role = portal.UserRoleAdmin

	for _, user := range users {
		userRepo.CreateUser(ctx, user)
	}

	// Test count all users
	count, err := userRepo.CountUsers(ctx, nil)
	if err != nil {
		t.Errorf("CountUsers() returned error: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected count 3, got %d", count)
	}

	// Test count with filter
	filter := &portal.UserFilter{Role: portal.UserRoleAdmin}
	count, err = userRepo.CountUsers(ctx, filter)
	if err != nil {
		t.Errorf("CountUsers() with filter returned error: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected count 1, got %d", count)
	}
}

func TestUserRepository_ExistsUserByEmail(t *testing.T) {
	repo := NewRepository()
	userRepo := NewUserRepository(repo)
	ctx := context.Background()

	user := createTestUser("user1", "test@example.com")
	userRepo.CreateUser(ctx, user)

	// Test existing email
	exists, err := userRepo.ExistsUserByEmail(ctx, "test@example.com")
	if err != nil {
		t.Errorf("ExistsUserByEmail() returned error: %v", err)
	}
	if !exists {
		t.Error("Email should exist")
	}

	// Test non-existent email
	exists, err = userRepo.ExistsUserByEmail(ctx, "nonexistent@example.com")
	if err != nil {
		t.Errorf("ExistsUserByEmail() returned error: %v", err)
	}
	if exists {
		t.Error("Email should not exist")
	}

	// Test empty email
	_, err = userRepo.ExistsUserByEmail(ctx, "")
	if err == nil {
		t.Error("Expected error for empty email")
	}
	if !portal.IsValidationError(err) {
		t.Errorf("Expected validation error, got: %v", err)
	}
}
