package memory

import (
	"context"
	"testing"

	"github.com/songzhibin97/stargate/pkg/portal"
)

func TestApplicationRepository_CreateApplication(t *testing.T) {
	repo := NewRepository()
	userRepo := NewUserRepository(repo)
	appRepo := NewApplicationRepository(repo)
	ctx := context.Background()

	// Create a test user first
	user := createTestUser("user1", "test@example.com")
	userRepo.CreateUser(ctx, user)

	app := createTestApplication("app1", "user1", "ak_test123")

	// Test successful creation
	err := appRepo.CreateApplication(ctx, app)
	if err != nil {
		t.Errorf("CreateApplication() returned error: %v", err)
	}

	// Verify application was stored
	storedApp, exists := repo.applications[app.ID]
	if !exists {
		t.Error("Application not stored in repository")
	}
	if storedApp.Name != app.Name {
		t.Errorf("Expected name %s, got %s", app.Name, storedApp.Name)
	}

	// Test duplicate ID
	duplicateApp := createTestApplication("app1", "user1", "ak_different")
	err = appRepo.CreateApplication(ctx, duplicateApp)
	if err == nil {
		t.Error("Expected error for duplicate application ID")
	}
	if !portal.IsConflictError(err) {
		t.Errorf("Expected conflict error, got: %v", err)
	}

	// Test duplicate API key
	duplicateKeyApp := createTestApplication("app2", "user1", "ak_test123")
	err = appRepo.CreateApplication(ctx, duplicateKeyApp)
	if err == nil {
		t.Error("Expected error for duplicate API key")
	}
	if !portal.IsConflictError(err) {
		t.Errorf("Expected conflict error, got: %v", err)
	}

	// Test non-existent user
	invalidUserApp := createTestApplication("app3", "nonexistent", "ak_test456")
	err = appRepo.CreateApplication(ctx, invalidUserApp)
	if err == nil {
		t.Error("Expected error for non-existent user")
	}
	if !portal.IsNotFoundError(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}

	// Test closed repository
	repo.Close()
	err = appRepo.CreateApplication(ctx, createTestApplication("app4", "user1", "ak_test789"))
	if err == nil {
		t.Error("Expected error for closed repository")
	}
	if !portal.IsDatabaseError(err) {
		t.Errorf("Expected database error, got: %v", err)
	}
}

func TestApplicationRepository_GetApplication(t *testing.T) {
	repo := NewRepository()
	userRepo := NewUserRepository(repo)
	appRepo := NewApplicationRepository(repo)
	ctx := context.Background()

	// Create test user and application
	user := createTestUser("user1", "test@example.com")
	userRepo.CreateUser(ctx, user)
	app := createTestApplication("app1", "user1", "ak_test123")
	appRepo.CreateApplication(ctx, app)

	// Test successful retrieval
	retrievedApp, err := appRepo.GetApplication(ctx, "app1")
	if err != nil {
		t.Errorf("GetApplication() returned error: %v", err)
	}
	if retrievedApp.ID != app.ID {
		t.Errorf("Expected ID %s, got %s", app.ID, retrievedApp.ID)
	}
	if retrievedApp.Name != app.Name {
		t.Errorf("Expected name %s, got %s", app.Name, retrievedApp.Name)
	}

	// Test non-existent application
	_, err = appRepo.GetApplication(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent application")
	}
	if !portal.IsNotFoundError(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}

	// Test empty ID
	_, err = appRepo.GetApplication(ctx, "")
	if err == nil {
		t.Error("Expected error for empty application ID")
	}
	if !portal.IsValidationError(err) {
		t.Errorf("Expected validation error, got: %v", err)
	}

	// Test closed repository
	repo.Close()
	_, err = appRepo.GetApplication(ctx, "app1")
	if err == nil {
		t.Error("Expected error for closed repository")
	}
	if !portal.IsDatabaseError(err) {
		t.Errorf("Expected database error, got: %v", err)
	}
}

func TestApplicationRepository_GetApplicationByAPIKey(t *testing.T) {
	repo := NewRepository()
	userRepo := NewUserRepository(repo)
	appRepo := NewApplicationRepository(repo)
	ctx := context.Background()

	// Create test user and application
	user := createTestUser("user1", "test@example.com")
	userRepo.CreateUser(ctx, user)
	app := createTestApplication("app1", "user1", "ak_test123")
	appRepo.CreateApplication(ctx, app)

	// Test successful retrieval
	retrievedApp, err := appRepo.GetApplicationByAPIKey(ctx, "ak_test123")
	if err != nil {
		t.Errorf("GetApplicationByAPIKey() returned error: %v", err)
	}
	if retrievedApp.ID != app.ID {
		t.Errorf("Expected ID %s, got %s", app.ID, retrievedApp.ID)
	}

	// Test non-existent API key
	_, err = appRepo.GetApplicationByAPIKey(ctx, "ak_nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent API key")
	}
	if !portal.IsNotFoundError(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}

	// Test empty API key
	_, err = appRepo.GetApplicationByAPIKey(ctx, "")
	if err == nil {
		t.Error("Expected error for empty API key")
	}
	if !portal.IsValidationError(err) {
		t.Errorf("Expected validation error, got: %v", err)
	}
}

func TestApplicationRepository_GetApplicationsByUser(t *testing.T) {
	repo := NewRepository()
	userRepo := NewUserRepository(repo)
	appRepo := NewApplicationRepository(repo)
	ctx := context.Background()

	// Create test users and applications
	user1 := createTestUser("user1", "user1@example.com")
	user2 := createTestUser("user2", "user2@example.com")
	userRepo.CreateUser(ctx, user1)
	userRepo.CreateUser(ctx, user2)

	app1 := createTestApplication("app1", "user1", "ak_test123")
	app2 := createTestApplication("app2", "user1", "ak_test456")
	app3 := createTestApplication("app3", "user2", "ak_test789")
	appRepo.CreateApplication(ctx, app1)
	appRepo.CreateApplication(ctx, app2)
	appRepo.CreateApplication(ctx, app3)

	// Test get applications for user1
	apps, err := appRepo.GetApplicationsByUser(ctx, "user1")
	if err != nil {
		t.Errorf("GetApplicationsByUser() returned error: %v", err)
	}
	if len(apps) != 2 {
		t.Errorf("Expected 2 applications for user1, got %d", len(apps))
	}

	// Test get applications for user2
	apps, err = appRepo.GetApplicationsByUser(ctx, "user2")
	if err != nil {
		t.Errorf("GetApplicationsByUser() returned error: %v", err)
	}
	if len(apps) != 1 {
		t.Errorf("Expected 1 application for user2, got %d", len(apps))
	}

	// Test get applications for user with no applications
	apps, err = appRepo.GetApplicationsByUser(ctx, "user3")
	if err != nil {
		t.Errorf("GetApplicationsByUser() returned error: %v", err)
	}
	if len(apps) != 0 {
		t.Errorf("Expected 0 applications for user3, got %d", len(apps))
	}

	// Test empty user ID
	_, err = appRepo.GetApplicationsByUser(ctx, "")
	if err == nil {
		t.Error("Expected error for empty user ID")
	}
	if !portal.IsValidationError(err) {
		t.Errorf("Expected validation error, got: %v", err)
	}
}

func TestApplicationRepository_UpdateApplication(t *testing.T) {
	repo := NewRepository()
	userRepo := NewUserRepository(repo)
	appRepo := NewApplicationRepository(repo)
	ctx := context.Background()

	// Create test user and application
	user := createTestUser("user1", "test@example.com")
	userRepo.CreateUser(ctx, user)
	app := createTestApplication("app1", "user1", "ak_test123")
	appRepo.CreateApplication(ctx, app)

	// Test successful update
	app.Name = "Updated App Name"
	app.Description = "Updated description"
	err := appRepo.UpdateApplication(ctx, app)
	if err != nil {
		t.Errorf("UpdateApplication() returned error: %v", err)
	}

	// Verify update
	updatedApp, _ := appRepo.GetApplication(ctx, "app1")
	if updatedApp.Name != "Updated App Name" {
		t.Errorf("Expected name 'Updated App Name', got '%s'", updatedApp.Name)
	}
	if updatedApp.Description != "Updated description" {
		t.Errorf("Expected description 'Updated description', got '%s'", updatedApp.Description)
	}

	// Test API key change
	app.APIKey = "ak_newkey123"
	err = appRepo.UpdateApplication(ctx, app)
	if err != nil {
		t.Errorf("UpdateApplication() with API key change returned error: %v", err)
	}

	// Verify API key index was updated
	_, err = appRepo.GetApplicationByAPIKey(ctx, "ak_test123")
	if err == nil {
		t.Error("Old API key should not exist in index")
	}
	_, err = appRepo.GetApplicationByAPIKey(ctx, "ak_newkey123")
	if err != nil {
		t.Error("New API key should exist in index")
	}

	// Test non-existent application
	nonExistentApp := createTestApplication("nonexistent", "user1", "ak_nonexistent")
	err = appRepo.UpdateApplication(ctx, nonExistentApp)
	if err == nil {
		t.Error("Expected error for non-existent application")
	}
	if !portal.IsNotFoundError(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}

	// Test API key conflict
	app2 := createTestApplication("app2", "user1", "ak_test456")
	appRepo.CreateApplication(ctx, app2)
	
	app2.APIKey = "ak_newkey123" // Same as app1's current API key
	err = appRepo.UpdateApplication(ctx, app2)
	if err == nil {
		t.Error("Expected error for API key conflict")
	}
	if !portal.IsConflictError(err) {
		t.Errorf("Expected conflict error, got: %v", err)
	}

	// Test non-existent user
	app.UserID = "nonexistent"
	err = appRepo.UpdateApplication(ctx, app)
	if err == nil {
		t.Error("Expected error for non-existent user")
	}
	if !portal.IsNotFoundError(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}
}

func TestApplicationRepository_DeleteApplication(t *testing.T) {
	repo := NewRepository()
	userRepo := NewUserRepository(repo)
	appRepo := NewApplicationRepository(repo)
	ctx := context.Background()

	// Create test user and application
	user := createTestUser("user1", "test@example.com")
	userRepo.CreateUser(ctx, user)
	app := createTestApplication("app1", "user1", "ak_test123")
	appRepo.CreateApplication(ctx, app)

	// Test successful deletion
	err := appRepo.DeleteApplication(ctx, "app1")
	if err != nil {
		t.Errorf("DeleteApplication() returned error: %v", err)
	}

	// Verify application was deleted
	_, err = appRepo.GetApplication(ctx, "app1")
	if err == nil {
		t.Error("Application should be deleted")
	}
	if !portal.IsNotFoundError(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}

	// Verify API key index was cleaned up
	_, err = appRepo.GetApplicationByAPIKey(ctx, "ak_test123")
	if err == nil {
		t.Error("API key should be removed from index")
	}

	// Verify user index was cleaned up
	apps, _ := appRepo.GetApplicationsByUser(ctx, "user1")
	if len(apps) != 0 {
		t.Error("Application should be removed from user index")
	}

	// Test non-existent application
	err = appRepo.DeleteApplication(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent application")
	}
	if !portal.IsNotFoundError(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}

	// Test empty ID
	err = appRepo.DeleteApplication(ctx, "")
	if err == nil {
		t.Error("Expected error for empty application ID")
	}
	if !portal.IsValidationError(err) {
		t.Errorf("Expected validation error, got: %v", err)
	}
}

func TestApplicationRepository_ExistsApplication(t *testing.T) {
	repo := NewRepository()
	userRepo := NewUserRepository(repo)
	appRepo := NewApplicationRepository(repo)
	ctx := context.Background()

	// Create test user and application
	user := createTestUser("user1", "test@example.com")
	userRepo.CreateUser(ctx, user)
	app := createTestApplication("app1", "user1", "ak_test123")
	appRepo.CreateApplication(ctx, app)

	// Test existing application
	exists, err := appRepo.ExistsApplication(ctx, "app1")
	if err != nil {
		t.Errorf("ExistsApplication() returned error: %v", err)
	}
	if !exists {
		t.Error("Application should exist")
	}

	// Test non-existent application
	exists, err = appRepo.ExistsApplication(ctx, "nonexistent")
	if err != nil {
		t.Errorf("ExistsApplication() returned error: %v", err)
	}
	if exists {
		t.Error("Application should not exist")
	}

	// Test empty ID
	_, err = appRepo.ExistsApplication(ctx, "")
	if err == nil {
		t.Error("Expected error for empty application ID")
	}
	if !portal.IsValidationError(err) {
		t.Errorf("Expected validation error, got: %v", err)
	}
}

func TestApplicationRepository_ExistsApplicationByAPIKey(t *testing.T) {
	repo := NewRepository()
	userRepo := NewUserRepository(repo)
	appRepo := NewApplicationRepository(repo)
	ctx := context.Background()

	// Create test user and application
	user := createTestUser("user1", "test@example.com")
	userRepo.CreateUser(ctx, user)
	app := createTestApplication("app1", "user1", "ak_test123")
	appRepo.CreateApplication(ctx, app)

	// Test existing API key
	exists, err := appRepo.ExistsApplicationByAPIKey(ctx, "ak_test123")
	if err != nil {
		t.Errorf("ExistsApplicationByAPIKey() returned error: %v", err)
	}
	if !exists {
		t.Error("API key should exist")
	}

	// Test non-existent API key
	exists, err = appRepo.ExistsApplicationByAPIKey(ctx, "ak_nonexistent")
	if err != nil {
		t.Errorf("ExistsApplicationByAPIKey() returned error: %v", err)
	}
	if exists {
		t.Error("API key should not exist")
	}

	// Test empty API key
	_, err = appRepo.ExistsApplicationByAPIKey(ctx, "")
	if err == nil {
		t.Error("Expected error for empty API key")
	}
	if !portal.IsValidationError(err) {
		t.Errorf("Expected validation error, got: %v", err)
	}
}

func TestApplicationRepository_UpdateApplicationStatus(t *testing.T) {
	repo := NewRepository()
	userRepo := NewUserRepository(repo)
	appRepo := NewApplicationRepository(repo)
	ctx := context.Background()

	// Create test user and application
	user := createTestUser("user1", "test@example.com")
	userRepo.CreateUser(ctx, user)
	app := createTestApplication("app1", "user1", "ak_test123")
	appRepo.CreateApplication(ctx, app)

	// Test successful status update
	err := appRepo.UpdateApplicationStatus(ctx, "app1", portal.ApplicationStatusSuspended)
	if err != nil {
		t.Errorf("UpdateApplicationStatus() returned error: %v", err)
	}

	// Verify status was updated
	updatedApp, _ := appRepo.GetApplication(ctx, "app1")
	if updatedApp.Status != portal.ApplicationStatusSuspended {
		t.Errorf("Expected status %s, got %s", portal.ApplicationStatusSuspended, updatedApp.Status)
	}

	// Test non-existent application
	err = appRepo.UpdateApplicationStatus(ctx, "nonexistent", portal.ApplicationStatusActive)
	if err == nil {
		t.Error("Expected error for non-existent application")
	}
	if !portal.IsNotFoundError(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}

	// Test empty ID
	err = appRepo.UpdateApplicationStatus(ctx, "", portal.ApplicationStatusActive)
	if err == nil {
		t.Error("Expected error for empty application ID")
	}
	if !portal.IsValidationError(err) {
		t.Errorf("Expected validation error, got: %v", err)
	}
}

func TestApplicationRepository_UpdateApplicationRateLimit(t *testing.T) {
	repo := NewRepository()
	userRepo := NewUserRepository(repo)
	appRepo := NewApplicationRepository(repo)
	ctx := context.Background()

	// Create test user and application
	user := createTestUser("user1", "test@example.com")
	userRepo.CreateUser(ctx, user)
	app := createTestApplication("app1", "user1", "ak_test123")
	appRepo.CreateApplication(ctx, app)

	// Test successful rate limit update
	err := appRepo.UpdateApplicationRateLimit(ctx, "app1", 2000)
	if err != nil {
		t.Errorf("UpdateApplicationRateLimit() returned error: %v", err)
	}

	// Verify rate limit was updated
	updatedApp, _ := appRepo.GetApplication(ctx, "app1")
	if updatedApp.RateLimit != 2000 {
		t.Errorf("Expected rate limit 2000, got %d", updatedApp.RateLimit)
	}

	// Test negative rate limit
	err = appRepo.UpdateApplicationRateLimit(ctx, "app1", -1)
	if err == nil {
		t.Error("Expected error for negative rate limit")
	}
	if !portal.IsValidationError(err) {
		t.Errorf("Expected validation error, got: %v", err)
	}

	// Test non-existent application
	err = appRepo.UpdateApplicationRateLimit(ctx, "nonexistent", 1000)
	if err == nil {
		t.Error("Expected error for non-existent application")
	}
	if !portal.IsNotFoundError(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}
}

func TestApplicationRepository_RegenerateAPIKey(t *testing.T) {
	repo := NewRepository()
	userRepo := NewUserRepository(repo)
	appRepo := NewApplicationRepository(repo)
	ctx := context.Background()

	// Create test user and application
	user := createTestUser("user1", "test@example.com")
	userRepo.CreateUser(ctx, user)
	app := createTestApplication("app1", "user1", "ak_test123")
	appRepo.CreateApplication(ctx, app)

	oldAPIKey := app.APIKey

	// Test successful API key regeneration
	newAPIKey, err := appRepo.RegenerateAPIKey(ctx, "app1")
	if err != nil {
		t.Errorf("RegenerateAPIKey() returned error: %v", err)
	}
	if newAPIKey == "" {
		t.Error("New API key should not be empty")
	}
	if newAPIKey == oldAPIKey {
		t.Error("New API key should be different from old one")
	}

	// Verify API key was updated in application
	updatedApp, _ := appRepo.GetApplication(ctx, "app1")
	if updatedApp.APIKey != newAPIKey {
		t.Errorf("Expected API key %s, got %s", newAPIKey, updatedApp.APIKey)
	}

	// Verify old API key is no longer valid
	_, err = appRepo.GetApplicationByAPIKey(ctx, oldAPIKey)
	if err == nil {
		t.Error("Old API key should not be valid")
	}

	// Verify new API key is valid
	_, err = appRepo.GetApplicationByAPIKey(ctx, newAPIKey)
	if err != nil {
		t.Error("New API key should be valid")
	}

	// Test non-existent application
	_, err = appRepo.RegenerateAPIKey(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent application")
	}
	if !portal.IsNotFoundError(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}
}

func TestApplicationRepository_RegenerateAPISecret(t *testing.T) {
	repo := NewRepository()
	userRepo := NewUserRepository(repo)
	appRepo := NewApplicationRepository(repo)
	ctx := context.Background()

	// Create test user and application
	user := createTestUser("user1", "test@example.com")
	userRepo.CreateUser(ctx, user)
	app := createTestApplication("app1", "user1", "ak_test123")
	appRepo.CreateApplication(ctx, app)

	oldAPISecret := app.APISecret

	// Test successful API secret regeneration
	newAPISecret, err := appRepo.RegenerateAPISecret(ctx, "app1")
	if err != nil {
		t.Errorf("RegenerateAPISecret() returned error: %v", err)
	}
	if newAPISecret == "" {
		t.Error("New API secret should not be empty")
	}
	if newAPISecret == oldAPISecret {
		t.Error("New API secret should be different from old one")
	}

	// Verify API secret was updated in application
	updatedApp, _ := appRepo.GetApplication(ctx, "app1")
	if updatedApp.APISecret != newAPISecret {
		t.Errorf("Expected API secret %s, got %s", newAPISecret, updatedApp.APISecret)
	}

	// Test non-existent application
	_, err = appRepo.RegenerateAPISecret(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent application")
	}
	if !portal.IsNotFoundError(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}
}

func TestApplicationRepository_CountApplicationsByUser(t *testing.T) {
	repo := NewRepository()
	userRepo := NewUserRepository(repo)
	appRepo := NewApplicationRepository(repo)
	ctx := context.Background()

	// Create test users and applications
	user1 := createTestUser("user1", "user1@example.com")
	user2 := createTestUser("user2", "user2@example.com")
	userRepo.CreateUser(ctx, user1)
	userRepo.CreateUser(ctx, user2)

	app1 := createTestApplication("app1", "user1", "ak_test123")
	app2 := createTestApplication("app2", "user1", "ak_test456")
	app3 := createTestApplication("app3", "user2", "ak_test789")
	appRepo.CreateApplication(ctx, app1)
	appRepo.CreateApplication(ctx, app2)
	appRepo.CreateApplication(ctx, app3)

	// Test count for user1
	count, err := appRepo.CountApplicationsByUser(ctx, "user1")
	if err != nil {
		t.Errorf("CountApplicationsByUser() returned error: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected count 2 for user1, got %d", count)
	}

	// Test count for user2
	count, err = appRepo.CountApplicationsByUser(ctx, "user2")
	if err != nil {
		t.Errorf("CountApplicationsByUser() returned error: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected count 1 for user2, got %d", count)
	}

	// Test count for user with no applications
	count, err = appRepo.CountApplicationsByUser(ctx, "user3")
	if err != nil {
		t.Errorf("CountApplicationsByUser() returned error: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected count 0 for user3, got %d", count)
	}

	// Test empty user ID
	_, err = appRepo.CountApplicationsByUser(ctx, "")
	if err == nil {
		t.Error("Expected error for empty user ID")
	}
	if !portal.IsValidationError(err) {
		t.Errorf("Expected validation error, got: %v", err)
	}
}
