package memory

import (
	"context"
	"testing"
	"time"

	"github.com/songzhibin97/stargate/pkg/portal"
)

func TestNewRepository(t *testing.T) {
	repo := NewRepository()
	if repo == nil {
		t.Fatal("NewRepository() returned nil")
	}

	if repo.users == nil {
		t.Error("users map not initialized")
	}
	if repo.applications == nil {
		t.Error("applications map not initialized")
	}
	if repo.usersByEmail == nil {
		t.Error("usersByEmail map not initialized")
	}
	if repo.appsByAPIKey == nil {
		t.Error("appsByAPIKey map not initialized")
	}
	if repo.appsByUser == nil {
		t.Error("appsByUser map not initialized")
	}
}

func TestRepository_Health(t *testing.T) {
	repo := NewRepository()
	ctx := context.Background()

	// Test healthy repository
	health := repo.Health(ctx)
	if health.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", health.Status)
	}
	if health.Message != "In-memory repository is operational" {
		t.Errorf("Unexpected health message: %s", health.Message)
	}

	// Test closed repository
	repo.Close()
	health = repo.Health(ctx)
	if health.Status != "unhealthy" {
		t.Errorf("Expected status 'unhealthy' for closed repo, got '%s'", health.Status)
	}
}

func TestRepository_Close(t *testing.T) {
	repo := NewRepository()

	// Add some test data
	user := &portal.User{
		ID:    "user1",
		Email: "test@example.com",
		Name:  "Test User",
		Role:  portal.UserRoleDeveloper,
		Status: portal.UserStatusActive,
	}
	repo.users["user1"] = user
	repo.usersByEmail["test@example.com"] = user

	// Close repository
	err := repo.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	if !repo.closed {
		t.Error("Repository not marked as closed")
	}

	// Test double close
	err = repo.Close()
	if err != nil {
		t.Errorf("Double close returned error: %v", err)
	}
}

func TestRepository_BeginTx(t *testing.T) {
	repo := NewRepository()
	ctx := context.Background()

	// Test successful transaction creation
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Errorf("BeginTx() returned error: %v", err)
	}
	if tx == nil {
		t.Error("BeginTx() returned nil transaction")
	}

	// Test transaction on closed repository
	repo.Close()
	tx, err = repo.BeginTx(ctx)
	if err == nil {
		t.Error("BeginTx() should return error for closed repository")
	}
	if !portal.IsDatabaseError(err) {
		t.Errorf("Expected database error, got: %v", err)
	}
}

func TestRepository_Validation(t *testing.T) {
	repo := NewRepository()

	// Test valid user
	validUser := &portal.User{
		ID:     "user1",
		Email:  "test@example.com",
		Name:   "Test User",
		Role:   portal.UserRoleDeveloper,
		Status: portal.UserStatusActive,
	}
	err := repo.isValidUser(validUser)
	if err != nil {
		t.Errorf("Valid user failed validation: %v", err)
	}

	// Test invalid users
	testCases := []struct {
		name string
		user *portal.User
	}{
		{"nil user", nil},
		{"empty ID", &portal.User{Email: "test@example.com", Name: "Test", Role: portal.UserRoleDeveloper, Status: portal.UserStatusActive}},
		{"empty email", &portal.User{ID: "user1", Name: "Test", Role: portal.UserRoleDeveloper, Status: portal.UserStatusActive}},
		{"empty name", &portal.User{ID: "user1", Email: "test@example.com", Role: portal.UserRoleDeveloper, Status: portal.UserStatusActive}},
		{"empty role", &portal.User{ID: "user1", Email: "test@example.com", Name: "Test", Status: portal.UserStatusActive}},
		{"empty status", &portal.User{ID: "user1", Email: "test@example.com", Name: "Test", Role: portal.UserRoleDeveloper}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := repo.isValidUser(tc.user)
			if err == nil {
				t.Error("Expected validation error")
			}
			if !portal.IsValidationError(err) {
				t.Errorf("Expected validation error, got: %v", err)
			}
		})
	}

	// Test valid application
	validApp := &portal.Application{
		ID:      "app1",
		Name:    "Test App",
		UserID:  "user1",
		APIKey:  "ak_test123",
		Status:  portal.ApplicationStatusActive,
	}
	err = repo.isValidApplication(validApp)
	if err != nil {
		t.Errorf("Valid application failed validation: %v", err)
	}

	// Test invalid applications
	appTestCases := []struct {
		name string
		app  *portal.Application
	}{
		{"nil app", nil},
		{"empty ID", &portal.Application{Name: "Test", UserID: "user1", APIKey: "ak_test", Status: portal.ApplicationStatusActive}},
		{"empty name", &portal.Application{ID: "app1", UserID: "user1", APIKey: "ak_test", Status: portal.ApplicationStatusActive}},
		{"empty user ID", &portal.Application{ID: "app1", Name: "Test", APIKey: "ak_test", Status: portal.ApplicationStatusActive}},
		{"empty API key", &portal.Application{ID: "app1", Name: "Test", UserID: "user1", Status: portal.ApplicationStatusActive}},
		{"empty status", &portal.Application{ID: "app1", Name: "Test", UserID: "user1", APIKey: "ak_test"}},
	}

	for _, tc := range appTestCases {
		t.Run(tc.name, func(t *testing.T) {
			err := repo.isValidApplication(tc.app)
			if err == nil {
				t.Error("Expected validation error")
			}
			if !portal.IsValidationError(err) {
				t.Errorf("Expected validation error, got: %v", err)
			}
		})
	}
}

func TestRepository_IndexManagement(t *testing.T) {
	repo := NewRepository()

	user := &portal.User{
		ID:    "user1",
		Email: "test@example.com",
		Name:  "Test User",
		Role:  portal.UserRoleDeveloper,
		Status: portal.UserStatusActive,
	}

	app := &portal.Application{
		ID:     "app1",
		Name:   "Test App",
		UserID: "user1",
		APIKey: "ak_test123",
		Status: portal.ApplicationStatusActive,
	}

	// Test user index management
	repo.addUserToIndex(user)
	if repo.usersByEmail[user.Email] != user {
		t.Error("User not added to email index")
	}

	repo.removeUserFromIndex(user)
	if _, exists := repo.usersByEmail[user.Email]; exists {
		t.Error("User not removed from email index")
	}

	// Test application index management
	repo.addApplicationToIndex(app)
	if repo.appsByAPIKey[app.APIKey] != app {
		t.Error("Application not added to API key index")
	}
	if len(repo.appsByUser[app.UserID]) != 1 || repo.appsByUser[app.UserID][0] != app {
		t.Error("Application not added to user index")
	}

	repo.removeApplicationFromIndex(app)
	if _, exists := repo.appsByAPIKey[app.APIKey]; exists {
		t.Error("Application not removed from API key index")
	}
	if _, exists := repo.appsByUser[app.UserID]; exists {
		t.Error("Application not removed from user index")
	}
}

func createTestUser(id, email string) *portal.User {
	return &portal.User{
		ID:        id,
		Email:     email,
		Name:      "Test User " + id,
		Role:      portal.UserRoleDeveloper,
		Status:    portal.UserStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func createTestApplication(id, userID, apiKey string) *portal.Application {
	return &portal.Application{
		ID:          id,
		Name:        "Test App " + id,
		Description: "Test application",
		UserID:      userID,
		APIKey:      apiKey,
		APISecret:   "as_secret123",
		Status:      portal.ApplicationStatusActive,
		RateLimit:   1000,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}
