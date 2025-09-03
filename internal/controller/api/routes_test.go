package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/router"
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

// MockConfigNotifier implements ConfigNotifier interface for testing
type MockConfigNotifier struct{}

func (m *MockConfigNotifier) PublishConfigChange(changeType string, key string, value, oldValue []byte, source string) error {
	return nil
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

// Define ErrKeyNotFound for the mock store
var ErrKeyNotFound = errors.New("key not found")

func TestRouteHandler_CreateRoute(t *testing.T) {
	// Setup
	cfg := &config.Config{}
	mockStore := NewMockStore()
	mockNotifier := &MockConfigNotifier{}
	handler := NewRouteHandler(cfg, mockStore, mockNotifier)

	// Test data
	route := router.RouteRule{
		ID:   "test-route",
		Name: "Test Route",
		Rules: router.Rule{
			Hosts: []string{"test.example.com"},
			Paths: []router.PathRule{
				{Type: router.MatchTypePrefix, Value: "/api"},
			},
			Methods: []string{"GET", "POST"},
		},
		UpstreamID: "test-upstream",
		Priority:   100,
	}

	// Convert to JSON
	jsonData, err := json.Marshal(route)
	if err != nil {
		t.Fatalf("Failed to marshal route: %v", err)
	}

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/routes", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.CreateRoute(w, req)

	// Verify response
	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}

	// Verify route was stored
	storedData, err := mockStore.Get(context.Background(), "routes/test-route")
	if err != nil {
		t.Errorf("Route was not stored: %v", err)
	}

	var storedRoute router.RouteRule
	if err := json.Unmarshal(storedData, &storedRoute); err != nil {
		t.Errorf("Failed to unmarshal stored route: %v", err)
	}

	if storedRoute.ID != route.ID {
		t.Errorf("Expected route ID %s, got %s", route.ID, storedRoute.ID)
	}
}

func TestRouteHandler_GetRoute(t *testing.T) {
	// Setup
	cfg := &config.Config{}
	mockStore := NewMockStore()
	mockNotifier := &MockConfigNotifier{}
	handler := NewRouteHandler(cfg, mockStore, mockNotifier)

	// Pre-populate store
	route := router.RouteRule{
		ID:   "test-route",
		Name: "Test Route",
		Rules: router.Rule{
			Hosts: []string{"test.example.com"},
		},
		UpstreamID: "test-upstream",
	}

	jsonData, _ := json.Marshal(route)
	mockStore.Put(context.Background(), "routes/test-route", jsonData)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/routes/test-route", nil)
	w := httptest.NewRecorder()

	// Execute
	handler.GetRoute(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var responseRoute router.RouteRule
	if err := json.NewDecoder(w.Body).Decode(&responseRoute); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if responseRoute.ID != route.ID {
		t.Errorf("Expected route ID %s, got %s", route.ID, responseRoute.ID)
	}
}

func TestRouteHandler_UpdateRoute(t *testing.T) {
	// Setup
	cfg := &config.Config{}
	mockStore := NewMockStore()
	mockNotifier := &MockConfigNotifier{}
	handler := NewRouteHandler(cfg, mockStore, mockNotifier)

	// Pre-populate store
	originalRoute := router.RouteRule{
		ID:   "test-route",
		Name: "Original Route",
		Rules: router.Rule{
			Hosts: []string{"original.example.com"},
		},
		UpstreamID: "test-upstream",
	}

	jsonData, _ := json.Marshal(originalRoute)
	mockStore.Put(context.Background(), "routes/test-route", jsonData)

	// Updated route
	updatedRoute := router.RouteRule{
		ID:   "test-route",
		Name: "Updated Route",
		Rules: router.Rule{
			Hosts: []string{"updated.example.com"},
		},
		UpstreamID: "test-upstream",
	}

	jsonData, _ = json.Marshal(updatedRoute)

	// Create request
	req := httptest.NewRequest(http.MethodPut, "/routes/test-route", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.UpdateRoute(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Verify route was updated
	storedData, err := mockStore.Get(context.Background(), "routes/test-route")
	if err != nil {
		t.Errorf("Route was not found: %v", err)
	}

	var storedRoute router.RouteRule
	if err := json.Unmarshal(storedData, &storedRoute); err != nil {
		t.Errorf("Failed to unmarshal stored route: %v", err)
	}

	if storedRoute.Name != updatedRoute.Name {
		t.Errorf("Expected route name %s, got %s", updatedRoute.Name, storedRoute.Name)
	}
}

func TestRouteHandler_DeleteRoute(t *testing.T) {
	// Setup
	cfg := &config.Config{}
	mockStore := NewMockStore()
	mockNotifier := &MockConfigNotifier{}
	handler := NewRouteHandler(cfg, mockStore, mockNotifier)

	// Pre-populate store
	route := router.RouteRule{
		ID:   "test-route",
		Name: "Test Route",
		Rules: router.Rule{
			Hosts: []string{"test.example.com"},
		},
		UpstreamID: "test-upstream",
	}

	jsonData, _ := json.Marshal(route)
	mockStore.Put(context.Background(), "routes/test-route", jsonData)

	// Create request
	req := httptest.NewRequest(http.MethodDelete, "/routes/test-route", nil)
	w := httptest.NewRecorder()

	// Execute
	handler.DeleteRoute(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Verify route was deleted
	_, err := mockStore.Get(context.Background(), "routes/test-route")
	if err == nil {
		t.Error("Route should have been deleted")
	}
}

func TestRouteHandler_ListRoutes(t *testing.T) {
	// Setup
	cfg := &config.Config{}
	mockStore := NewMockStore()
	mockNotifier := &MockConfigNotifier{}
	handler := NewRouteHandler(cfg, mockStore, mockNotifier)

	// Pre-populate store with multiple routes
	routes := []router.RouteRule{
		{
			ID:   "route-1",
			Name: "Route 1",
			Rules: router.Rule{
				Hosts: []string{"route1.example.com"},
			},
			UpstreamID: "upstream-1",
		},
		{
			ID:   "route-2",
			Name: "Route 2",
			Rules: router.Rule{
				Hosts: []string{"route2.example.com"},
			},
			UpstreamID: "upstream-2",
		},
	}

	for _, route := range routes {
		jsonData, _ := json.Marshal(route)
		mockStore.Put(context.Background(), "routes/"+route.ID, jsonData)
	}

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/routes", nil)
	w := httptest.NewRecorder()

	// Execute
	handler.ListRoutes(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	routesData, ok := response["routes"].([]interface{})
	if !ok {
		t.Error("Response should contain routes array")
	}

	if len(routesData) != 2 {
		t.Errorf("Expected 2 routes, got %d", len(routesData))
	}

	total, ok := response["total"].(float64)
	if !ok || int(total) != 2 {
		t.Errorf("Expected total 2, got %v", total)
	}
}
