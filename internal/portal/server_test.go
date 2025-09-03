package portal

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/store"
)

// MockStore implements store.Store interface for testing
type MockStore struct {
	data map[string][]byte
}

func NewMockStore() *MockStore {
	return &MockStore{
		data: make(map[string][]byte),
	}
}

func (ms *MockStore) Get(ctx context.Context, key string) ([]byte, error) {
	if data, exists := ms.data[key]; exists {
		return data, nil
	}
	return nil, store.ErrKeyNotFound
}

func (ms *MockStore) Put(ctx context.Context, key string, value []byte) error {
	ms.data[key] = value
	return nil
}

func (ms *MockStore) Delete(ctx context.Context, key string) error {
	delete(ms.data, key)
	return nil
}

func (ms *MockStore) List(ctx context.Context, prefix string) (map[string][]byte, error) {
	result := make(map[string][]byte)
	for key, value := range ms.data {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			result[key] = value
		}
	}
	return result, nil
}

func (ms *MockStore) Watch(key string, callback store.WatchCallback) error {
	return nil
}

func (ms *MockStore) Unwatch(key string) error {
	return nil
}

func (ms *MockStore) Close() error {
	return nil
}

func TestServer_Health(t *testing.T) {
	// Setup
	cfg := &config.Config{
		Portal: config.PortalConfig{
			Port: 8080,
		},
	}
	mockStore := NewMockStore()
	
	server, err := NewServer(cfg, mockStore)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Create test request
	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()

	// Execute
	server.handleHealth(w, req)

	// Verify
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got %v", response["status"])
	}

	if response["version"] != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %v", response["version"])
	}
}

func TestServer_Login(t *testing.T) {
	// Setup
	cfg := &config.Config{
		Portal: config.PortalConfig{
			Port: 8080,
		},
	}
	mockStore := NewMockStore()
	
	server, err := NewServer(cfg, mockStore)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Create test request
	loginData := map[string]string{
		"username": "testuser",
		"password": "testpass",
	}
	jsonData, _ := json.Marshal(loginData)
	
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	server.handleLogin(w, req)

	// Verify
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["token"] == nil {
		t.Error("Expected token in response")
	}

	if response["refresh_token"] == nil {
		t.Error("Expected refresh_token in response")
	}

	if user, ok := response["user"].(map[string]interface{}); ok {
		if user["id"] == nil {
			t.Error("Expected user.id in response")
		}
	} else {
		t.Error("Expected user object in response")
	}
}

func TestServer_GetAPIs(t *testing.T) {
	// Setup
	cfg := &config.Config{
		Portal: config.PortalConfig{
			Port: 8080,
		},
	}
	mockStore := NewMockStore()
	
	// Add test data
	testSpec := &CachedSpec{
		RouteID:     "test-api",
		URL:         "http://example.com/openapi.json",
		Content: map[string]interface{}{
			"info": map[string]interface{}{
				"title":       "Test API",
				"description": "A test API",
				"version":     "1.0.0",
			},
			"paths": map[string]interface{}{
				"/test": map[string]interface{}{
					"get": map[string]interface{}{
						"summary": "Test endpoint",
					},
				},
			},
		},
		LastFetched: time.Now().Unix(),
	}
	
	specData, _ := json.Marshal(testSpec)
	mockStore.Put(context.Background(), "portal/specs/test-api", specData)
	
	server, err := NewServer(cfg, mockStore)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Start doc fetcher to load cached specs
	server.docFetcher.Start()
	defer server.docFetcher.Stop()
	
	// Wait for specs to load
	time.Sleep(100 * time.Millisecond)

	// Create test request with auth
	req := httptest.NewRequest("GET", "/api/v1/portal/apis", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	// Execute
	server.handleGetAPIs(w, req)

	// Verify
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	apis, ok := response["apis"].([]interface{})
	if !ok {
		t.Fatal("Expected apis array in response")
	}

	if len(apis) != 1 {
		t.Errorf("Expected 1 API, got %d", len(apis))
	}

	if total, ok := response["total"].(float64); !ok || int(total) != 1 {
		t.Errorf("Expected total 1, got %v", response["total"])
	}
}

func TestServer_GetAPIDetail(t *testing.T) {
	// Setup
	cfg := &config.Config{
		Portal: config.PortalConfig{
			Port: 8080,
		},
	}
	mockStore := NewMockStore()
	
	// Add test data
	testSpec := &CachedSpec{
		RouteID: "test-api",
		URL:     "http://example.com/openapi.json",
		Content: map[string]interface{}{
			"info": map[string]interface{}{
				"title":       "Test API",
				"description": "A test API",
				"version":     "1.0.0",
			},
			"paths": map[string]interface{}{
				"/test": map[string]interface{}{
					"get": map[string]interface{}{
						"summary": "Test endpoint",
					},
				},
			},
		},
		LastFetched: time.Now().Unix(),
	}
	
	server, err := NewServer(cfg, mockStore)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Manually add spec to cache
	server.docFetcher.cache["test-api"] = testSpec

	// Create test request with auth
	req := httptest.NewRequest("GET", "/api/v1/portal/apis/test-api", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	// Execute
	server.handleGetAPIDetail(w, req)

	// Verify
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response ParsedAPIInfo
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.RouteID != "test-api" {
		t.Errorf("Expected route_id 'test-api', got %s", response.RouteID)
	}

	if response.Title != "Test API" {
		t.Errorf("Expected title 'Test API', got %s", response.Title)
	}

	if response.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %s", response.Version)
	}
}

func TestServer_AuthMiddleware(t *testing.T) {
	// Setup
	cfg := &config.Config{
		Portal: config.PortalConfig{
			Port: 8080,
		},
	}
	mockStore := NewMockStore()
	
	server, err := NewServer(cfg, mockStore)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Test handler that requires auth
	testHandler := server.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("authenticated"))
	})

	// Test without auth header
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	testHandler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}

	// Test with auth header
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w = httptest.NewRecorder()
	testHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Body.String() != "authenticated" {
		t.Errorf("Expected 'authenticated', got %s", w.Body.String())
	}
}

func TestServer_CORS(t *testing.T) {
	// Setup
	cfg := &config.Config{
		Portal: config.PortalConfig{
			Port: 8080,
		},
	}
	mockStore := NewMockStore()
	
	server, err := NewServer(cfg, mockStore)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Test CORS headers
	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()

	// Use CORS middleware
	corsHandler := server.corsMiddleware(server.handleHealth)
	corsHandler(w, req)

	// Verify CORS headers
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Expected CORS Allow-Origin header")
	}

	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("Expected CORS Allow-Methods header")
	}

	if w.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Error("Expected CORS Allow-Headers header")
	}

	// Test OPTIONS request
	req = httptest.NewRequest("OPTIONS", "/api/v1/health", nil)
	w = httptest.NewRecorder()

	corsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for OPTIONS, got %d", w.Code)
	}
}

func TestServer_Dashboard(t *testing.T) {
	// Setup
	cfg := &config.Config{
		Portal: config.PortalConfig{
			Port: 8080,
		},
	}
	mockStore := NewMockStore()
	
	server, err := NewServer(cfg, mockStore)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Create test request with auth
	req := httptest.NewRequest("GET", "/api/v1/portal/dashboard", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	// Execute
	server.handleDashboard(w, req)

	// Verify
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Check required fields
	requiredFields := []string{"total_apis", "total_endpoints", "recent_tests", "uptime"}
	for _, field := range requiredFields {
		if _, exists := response[field]; !exists {
			t.Errorf("Expected field %s in dashboard response", field)
		}
	}
}
