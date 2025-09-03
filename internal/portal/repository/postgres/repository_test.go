package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/songzhibin97/stargate/pkg/portal"
)

var testRepo *Repository

func TestMain(m *testing.M) {
	// Setup test database
	if err := setupTestDB(); err != nil {
		fmt.Printf("Failed to setup test database: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	if testRepo != nil {
		testRepo.Close()
	}

	os.Exit(code)
}

func setupTestDB() error {
	// Use environment variable or default test database
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:password@localhost:5432/stargate_test?sslmode=disable"
	}

	config := &Config{
		DSN:             dsn,
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: 5 * time.Minute,
		MigrationPath:   "file://migrations",
	}

	var err error
	testRepo, err = NewRepository(config)
	if err != nil {
		return fmt.Errorf("failed to create repository: %w", err)
	}

	// Run migrations
	if err := testRepo.Migrate(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

func cleanupTestData(t *testing.T) {
	ctx := context.Background()
	
	// Clean up test data in reverse dependency order
	_, err := testRepo.db.ExecContext(ctx, "DELETE FROM api_usage_logs")
	if err != nil {
		t.Logf("Failed to clean api_usage_logs: %v", err)
	}
	
	_, err = testRepo.db.ExecContext(ctx, "DELETE FROM credentials")
	if err != nil {
		t.Logf("Failed to clean credentials: %v", err)
	}
	
	_, err = testRepo.db.ExecContext(ctx, "DELETE FROM applications")
	if err != nil {
		t.Logf("Failed to clean applications: %v", err)
	}
	
	_, err = testRepo.db.ExecContext(ctx, "DELETE FROM users WHERE id NOT IN ('admin-001', 'dev-001', 'viewer-001')")
	if err != nil {
		t.Logf("Failed to clean users: %v", err)
	}
}

func TestRepository_Health(t *testing.T) {
	if testRepo == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	health := testRepo.Health(ctx)

	if health.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", health.Status)
	}

	if health.Details == nil {
		t.Error("Expected health details to be present")
	}

	// Check required details
	details := health.Details
	if details["database_type"] != "postgresql" {
		t.Errorf("Expected database_type 'postgresql', got '%v'", details["database_type"])
	}

	if details["max_open_conns"] != testRepo.maxOpenConns {
		t.Errorf("Expected max_open_conns %d, got %v", testRepo.maxOpenConns, details["max_open_conns"])
	}
}

func TestRepository_BeginTx(t *testing.T) {
	if testRepo == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()

	// Test successful transaction creation
	tx, err := testRepo.BeginTx(ctx)
	if err != nil {
		t.Errorf("BeginTx() returned error: %v", err)
	}
	if tx == nil {
		t.Error("BeginTx() returned nil transaction")
	}

	// Test transaction commit
	err = tx.Commit(ctx)
	if err != nil {
		t.Errorf("Transaction commit failed: %v", err)
	}

	// Test transaction rollback
	tx2, err := testRepo.BeginTx(ctx)
	if err != nil {
		t.Errorf("BeginTx() returned error: %v", err)
	}

	err = tx2.Rollback(ctx)
	if err != nil {
		t.Errorf("Transaction rollback failed: %v", err)
	}
}

func TestRepository_UserRepository(t *testing.T) {
	if testRepo == nil {
		t.Skip("Test database not available")
	}

	defer cleanupTestData(t)

	ctx := context.Background()
	userRepo := NewUserRepository(testRepo)

	// Test CreateUser
	user := &portal.User{
		ID:     "test-user-1",
		Email:  "test1@example.com",
		Name:   "Test User 1",
		Role:   portal.UserRoleDeveloper,
		Status: portal.UserStatusActive,
	}

	err := userRepo.CreateUser(ctx, user)
	if err != nil {
		t.Errorf("CreateUser() returned error: %v", err)
	}

	// Test GetUser
	retrievedUser, err := userRepo.GetUser(ctx, "test-user-1")
	if err != nil {
		t.Errorf("GetUser() returned error: %v", err)
	}
	if retrievedUser.Email != user.Email {
		t.Errorf("Expected email %s, got %s", user.Email, retrievedUser.Email)
	}

	// Test GetUserByEmail
	userByEmail, err := userRepo.GetUserByEmail(ctx, "test1@example.com")
	if err != nil {
		t.Errorf("GetUserByEmail() returned error: %v", err)
	}
	if userByEmail.ID != user.ID {
		t.Errorf("Expected ID %s, got %s", user.ID, userByEmail.ID)
	}

	// Test UpdateUser
	user.Name = "Updated Test User 1"
	err = userRepo.UpdateUser(ctx, user)
	if err != nil {
		t.Errorf("UpdateUser() returned error: %v", err)
	}

	updatedUser, _ := userRepo.GetUser(ctx, "test-user-1")
	if updatedUser.Name != "Updated Test User 1" {
		t.Errorf("Expected name 'Updated Test User 1', got '%s'", updatedUser.Name)
	}

	// Test ExistsUser
	exists, err := userRepo.ExistsUser(ctx, "test-user-1")
	if err != nil {
		t.Errorf("ExistsUser() returned error: %v", err)
	}
	if !exists {
		t.Error("User should exist")
	}

	// Test DeleteUser
	err = userRepo.DeleteUser(ctx, "test-user-1")
	if err != nil {
		t.Errorf("DeleteUser() returned error: %v", err)
	}

	// Verify user was deleted
	_, err = userRepo.GetUser(ctx, "test-user-1")
	if err == nil {
		t.Error("User should be deleted")
	}
	if !portal.IsNotFoundError(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}
}

func TestRepository_ApplicationRepository(t *testing.T) {
	if testRepo == nil {
		t.Skip("Test database not available")
	}

	defer cleanupTestData(t)

	ctx := context.Background()
	userRepo := NewUserRepository(testRepo)
	appRepo := NewApplicationRepository(testRepo)

	// Create a test user first
	user := &portal.User{
		ID:     "test-user-2",
		Email:  "test2@example.com",
		Name:   "Test User 2",
		Role:   portal.UserRoleDeveloper,
		Status: portal.UserStatusActive,
	}
	err := userRepo.CreateUser(ctx, user)
	if err != nil {
		t.Errorf("CreateUser() returned error: %v", err)
	}

	// Test CreateApplication
	app := &portal.Application{
		ID:          "test-app-1",
		Name:        "Test App 1",
		Description: "Test application",
		UserID:      "test-user-2",
		APIKey:      "ak_test123456789",
		APISecret:   "as_secret123456789",
		Status:      portal.ApplicationStatusActive,
		RateLimit:   1000,
	}

	err = appRepo.CreateApplication(ctx, app)
	if err != nil {
		t.Errorf("CreateApplication() returned error: %v", err)
	}

	// Test GetApplication
	retrievedApp, err := appRepo.GetApplication(ctx, "test-app-1")
	if err != nil {
		t.Errorf("GetApplication() returned error: %v", err)
	}
	if retrievedApp.Name != app.Name {
		t.Errorf("Expected name %s, got %s", app.Name, retrievedApp.Name)
	}

	// Test GetApplicationByAPIKey
	appByAPIKey, err := appRepo.GetApplicationByAPIKey(ctx, "ak_test123456789")
	if err != nil {
		t.Errorf("GetApplicationByAPIKey() returned error: %v", err)
	}
	if appByAPIKey.ID != app.ID {
		t.Errorf("Expected ID %s, got %s", app.ID, appByAPIKey.ID)
	}

	// Test GetApplicationsByUser
	userApps, err := appRepo.GetApplicationsByUser(ctx, "test-user-2")
	if err != nil {
		t.Errorf("GetApplicationsByUser() returned error: %v", err)
	}
	if len(userApps) != 1 {
		t.Errorf("Expected 1 application, got %d", len(userApps))
	}

	// Test UpdateApplication
	app.Name = "Updated Test App 1"
	err = appRepo.UpdateApplication(ctx, app)
	if err != nil {
		t.Errorf("UpdateApplication() returned error: %v", err)
	}

	updatedApp, _ := appRepo.GetApplication(ctx, "test-app-1")
	if updatedApp.Name != "Updated Test App 1" {
		t.Errorf("Expected name 'Updated Test App 1', got '%s'", updatedApp.Name)
	}

	// Test RegenerateAPIKey
	newAPIKey, err := appRepo.RegenerateAPIKey(ctx, "test-app-1")
	if err != nil {
		t.Errorf("RegenerateAPIKey() returned error: %v", err)
	}
	if newAPIKey == app.APIKey {
		t.Error("New API key should be different from old one")
	}

	// Test DeleteApplication
	err = appRepo.DeleteApplication(ctx, "test-app-1")
	if err != nil {
		t.Errorf("DeleteApplication() returned error: %v", err)
	}

	// Verify application was deleted
	_, err = appRepo.GetApplication(ctx, "test-app-1")
	if err == nil {
		t.Error("Application should be deleted")
	}
	if !portal.IsNotFoundError(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}
}

func TestRepository_Transaction(t *testing.T) {
	if testRepo == nil {
		t.Skip("Test database not available")
	}

	defer cleanupTestData(t)

	ctx := context.Background()

	// Test transaction with commit
	tx, err := testRepo.BeginTx(ctx)
	if err != nil {
		t.Errorf("BeginTx() returned error: %v", err)
	}

	userRepo := tx.UserRepository()
	user := &portal.User{
		ID:     "tx-user-1",
		Email:  "tx1@example.com",
		Name:   "Transaction User 1",
		Role:   portal.UserRoleDeveloper,
		Status: portal.UserStatusActive,
	}

	err = userRepo.CreateUser(ctx, user)
	if err != nil {
		t.Errorf("CreateUser() in transaction returned error: %v", err)
	}

	// Commit transaction
	err = tx.Commit(ctx)
	if err != nil {
		t.Errorf("Transaction commit failed: %v", err)
	}

	// Verify user exists after commit
	directUserRepo := NewUserRepository(testRepo)
	_, err = directUserRepo.GetUser(ctx, "tx-user-1")
	if err != nil {
		t.Errorf("User should exist after transaction commit: %v", err)
	}

	// Test transaction with rollback
	tx2, err := testRepo.BeginTx(ctx)
	if err != nil {
		t.Errorf("BeginTx() returned error: %v", err)
	}

	userRepo2 := tx2.UserRepository()
	user2 := &portal.User{
		ID:     "tx-user-2",
		Email:  "tx2@example.com",
		Name:   "Transaction User 2",
		Role:   portal.UserRoleDeveloper,
		Status: portal.UserStatusActive,
	}

	err = userRepo2.CreateUser(ctx, user2)
	if err != nil {
		t.Errorf("CreateUser() in transaction returned error: %v", err)
	}

	// Rollback transaction
	err = tx2.Rollback(ctx)
	if err != nil {
		t.Errorf("Transaction rollback failed: %v", err)
	}

	// Verify user does not exist after rollback
	_, err = directUserRepo.GetUser(ctx, "tx-user-2")
	if err == nil {
		t.Error("User should not exist after transaction rollback")
	}
	if !portal.IsNotFoundError(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}
}
