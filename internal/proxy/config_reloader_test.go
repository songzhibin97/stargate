package proxy

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/router"
	"github.com/songzhibin97/stargate/internal/store"
)

// MockPipelineInterface defines the interface that MockPipeline implements
type MockPipelineInterface interface {
	UpdateRoute(route *router.RouteRule) error
	DeleteRoute(routeID string) error
	UpdateUpstream(upstream *router.Upstream) error
	DeleteUpstream(upstreamID string) error
	RebuildMiddleware() error
	ReloadRoutes(routes []router.RouteRule) error
	ReloadUpstreams(upstreams []router.Upstream) error
	// Test helper methods
	GetRoutes() map[string]*router.RouteRule
	GetUpstreams() map[string]*router.Upstream
	IsRebuilt() bool
}

// MockPipeline implements the pipeline interface for testing
type MockPipeline struct {
	mu        sync.RWMutex
	routes    map[string]*router.RouteRule
	upstreams map[string]*router.Upstream
	rebuilt   bool
}

func NewMockPipeline() MockPipelineInterface {
	return &MockPipeline{
		routes:    make(map[string]*router.RouteRule),
		upstreams: make(map[string]*router.Upstream),
	}
}

func (mp *MockPipeline) UpdateRoute(route *router.RouteRule) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.routes[route.ID] = route
	return nil
}

func (mp *MockPipeline) DeleteRoute(routeID string) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	delete(mp.routes, routeID)
	return nil
}

func (mp *MockPipeline) UpdateUpstream(upstream *router.Upstream) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.upstreams[upstream.ID] = upstream
	return nil
}

func (mp *MockPipeline) DeleteUpstream(upstreamID string) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	delete(mp.upstreams, upstreamID)
	return nil
}

func (mp *MockPipeline) RebuildMiddleware() error {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.rebuilt = true
	return nil
}

func (mp *MockPipeline) ReloadRoutes(routes []router.RouteRule) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.routes = make(map[string]*router.RouteRule)
	for _, route := range routes {
		mp.routes[route.ID] = &route
	}
	return nil
}

func (mp *MockPipeline) ReloadUpstreams(upstreams []router.Upstream) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.upstreams = make(map[string]*router.Upstream)
	for _, upstream := range upstreams {
		mp.upstreams[upstream.ID] = &upstream
	}
	return nil
}

// Test helper methods
func (mp *MockPipeline) GetRoutes() map[string]*router.RouteRule {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	result := make(map[string]*router.RouteRule)
	for k, v := range mp.routes {
		result[k] = v
	}
	return result
}

func (mp *MockPipeline) GetUpstreams() map[string]*router.Upstream {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	result := make(map[string]*router.Upstream)
	for k, v := range mp.upstreams {
		result[k] = v
	}
	return result
}

func (mp *MockPipeline) IsRebuilt() bool {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	return mp.rebuilt
}

// MockStore implements store.Store interface for testing
type MockStore struct {
	data      map[string][]byte
	watchers  map[string]store.WatchCallback
	watchKeys map[string]bool
}

func NewMockStore() *MockStore {
	return &MockStore{
		data:      make(map[string][]byte),
		watchers:  make(map[string]store.WatchCallback),
		watchKeys: make(map[string]bool),
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
	
	// Trigger watchers
	for watchKey, callback := range ms.watchers {
		if len(key) >= len(watchKey) && key[:len(watchKey)] == watchKey {
			go callback(key, value, store.EventTypePut)
		}
	}
	
	return nil
}

func (ms *MockStore) Delete(ctx context.Context, key string) error {
	oldValue := ms.data[key]
	delete(ms.data, key)
	
	// Trigger watchers
	for watchKey, callback := range ms.watchers {
		if len(key) >= len(watchKey) && key[:len(watchKey)] == watchKey {
			go callback(key, oldValue, store.EventTypeDelete)
		}
	}
	
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
	ms.watchers[key] = callback
	ms.watchKeys[key] = true
	return nil
}

func (ms *MockStore) Unwatch(key string) error {
	delete(ms.watchers, key)
	delete(ms.watchKeys, key)
	return nil
}

func (ms *MockStore) Close() error {
	return nil
}

func TestConfigReloader_RouteChanges(t *testing.T) {
	// Setup
	cfg := &config.Config{}
	mockStore := NewMockStore()
	mockPipeline := NewMockPipeline()
	
	reloader := NewConfigReloader(cfg, mockStore, mockPipeline)
	
	// Start the reloader
	if err := reloader.Start(); err != nil {
		t.Fatalf("Failed to start config reloader: %v", err)
	}
	defer reloader.Stop()
	
	// Test route creation
	route := router.RouteRule{
		ID:   "test-route",
		Name: "Test Route",
		Rules: router.Rule{
			Hosts: []string{"test.example.com"},
			Paths: []router.PathRule{
				{Type: router.MatchTypePrefix, Value: "/api"},
			},
		},
		UpstreamID: "test-upstream",
	}
	
	routeData, err := json.Marshal(route)
	if err != nil {
		t.Fatalf("Failed to marshal route: %v", err)
	}
	
	// Simulate route creation via store
	ctx := context.Background()
	if err := mockStore.Put(ctx, "routes/test-route", routeData); err != nil {
		t.Fatalf("Failed to put route: %v", err)
	}
	
	// Wait for the change to be processed
	time.Sleep(100 * time.Millisecond)
	
	// Verify route was updated in pipeline
	routes := mockPipeline.GetRoutes()
	if _, exists := routes["test-route"]; !exists {
		t.Error("Route was not updated in pipeline")
	}

	// Test route deletion
	if err := mockStore.Delete(ctx, "routes/test-route"); err != nil {
		t.Fatalf("Failed to delete route: %v", err)
	}

	// Wait for the change to be processed
	time.Sleep(100 * time.Millisecond)

	// Verify route was deleted from pipeline
	routes = mockPipeline.GetRoutes()
	if _, exists := routes["test-route"]; exists {
		t.Error("Route was not deleted from pipeline")
	}
}

func TestConfigReloader_UpstreamChanges(t *testing.T) {
	// Setup
	cfg := &config.Config{}
	mockStore := NewMockStore()
	mockPipeline := NewMockPipeline()
	
	reloader := NewConfigReloader(cfg, mockStore, mockPipeline)
	
	// Start the reloader
	if err := reloader.Start(); err != nil {
		t.Fatalf("Failed to start config reloader: %v", err)
	}
	defer reloader.Stop()
	
	// Test upstream creation
	upstream := router.Upstream{
		ID:   "test-upstream",
		Name: "Test Upstream",
		Targets: []router.Target{
			{URL: "http://backend1:8080", Weight: 100},
			{URL: "http://backend2:8080", Weight: 100},
		},
		Algorithm: "round_robin",
	}
	
	upstreamData, err := json.Marshal(upstream)
	if err != nil {
		t.Fatalf("Failed to marshal upstream: %v", err)
	}
	
	// Simulate upstream creation via store
	ctx := context.Background()
	if err := mockStore.Put(ctx, "upstreams/test-upstream", upstreamData); err != nil {
		t.Fatalf("Failed to put upstream: %v", err)
	}
	
	// Wait for the change to be processed
	time.Sleep(100 * time.Millisecond)
	
	// Verify upstream was updated in pipeline
	upstreams := mockPipeline.GetUpstreams()
	if _, exists := upstreams["test-upstream"]; !exists {
		t.Error("Upstream was not updated in pipeline")
	}

	// Test upstream deletion
	if err := mockStore.Delete(ctx, "upstreams/test-upstream"); err != nil {
		t.Fatalf("Failed to delete upstream: %v", err)
	}

	// Wait for the change to be processed
	time.Sleep(100 * time.Millisecond)

	// Verify upstream was deleted from pipeline
	upstreams = mockPipeline.GetUpstreams()
	if _, exists := upstreams["test-upstream"]; exists {
		t.Error("Upstream was not deleted from pipeline")
	}
}

func TestConfigReloader_PluginChanges(t *testing.T) {
	// Setup
	cfg := &config.Config{}
	mockStore := NewMockStore()
	mockPipeline := NewMockPipeline()
	
	reloader := NewConfigReloader(cfg, mockStore, mockPipeline)
	
	// Start the reloader
	if err := reloader.Start(); err != nil {
		t.Fatalf("Failed to start config reloader: %v", err)
	}
	defer reloader.Stop()
	
	// Test plugin creation (should trigger middleware rebuild)
	plugin := map[string]interface{}{
		"id":      "test-plugin",
		"name":    "Test Plugin",
		"type":    "rate_limit",
		"enabled": true,
		"config": map[string]interface{}{
			"max_requests": 100,
			"window_size":  "1m",
		},
	}
	
	pluginData, err := json.Marshal(plugin)
	if err != nil {
		t.Fatalf("Failed to marshal plugin: %v", err)
	}
	
	// Simulate plugin creation via store
	ctx := context.Background()
	if err := mockStore.Put(ctx, "plugins/test-plugin", pluginData); err != nil {
		t.Fatalf("Failed to put plugin: %v", err)
	}

	// Wait for the change to be processed
	time.Sleep(100 * time.Millisecond)

	// Verify middleware was rebuilt
	if !mockPipeline.IsRebuilt() {
		t.Error("Middleware was not rebuilt after plugin change")
	}
}

func TestConfigReloader_FullReload(t *testing.T) {
	// Setup
	cfg := &config.Config{}
	mockStore := NewMockStore()
	mockPipeline := NewMockPipeline()
	
	reloader := NewConfigReloader(cfg, mockStore, mockPipeline)
	
	// Pre-populate store with test data
	ctx := context.Background()
	
	// Add test routes
	route1 := router.RouteRule{ID: "route1", Name: "Route 1", UpstreamID: "upstream1"}
	route2 := router.RouteRule{ID: "route2", Name: "Route 2", UpstreamID: "upstream2"}
	
	route1Data, _ := json.Marshal(route1)
	route2Data, _ := json.Marshal(route2)
	
	mockStore.Put(ctx, "routes/route1", route1Data)
	mockStore.Put(ctx, "routes/route2", route2Data)
	
	// Add test upstreams
	upstream1 := router.Upstream{ID: "upstream1", Name: "Upstream 1"}
	upstream2 := router.Upstream{ID: "upstream2", Name: "Upstream 2"}
	
	upstream1Data, _ := json.Marshal(upstream1)
	upstream2Data, _ := json.Marshal(upstream2)
	
	mockStore.Put(ctx, "upstreams/upstream1", upstream1Data)
	mockStore.Put(ctx, "upstreams/upstream2", upstream2Data)

	// Start the reloader first
	if err := reloader.Start(); err != nil {
		t.Fatalf("Failed to start config reloader: %v", err)
	}
	defer reloader.Stop()

	// Perform full reload
	reloader.performFullReload()
	
	// Verify all routes were loaded
	routes := mockPipeline.GetRoutes()
	if len(routes) != 2 {
		t.Errorf("Expected 2 routes, got %d", len(routes))
	}

	// Verify all upstreams were loaded
	upstreams := mockPipeline.GetUpstreams()
	if len(upstreams) != 2 {
		t.Errorf("Expected 2 upstreams, got %d", len(upstreams))
	}
}

func TestConfigReloader_Status(t *testing.T) {
	// Setup
	cfg := &config.Config{}
	mockStore := NewMockStore()
	mockPipeline := NewMockPipeline()
	
	reloader := NewConfigReloader(cfg, mockStore, mockPipeline)
	
	// Test status before starting
	status := reloader.GetStatus()
	if status["running"].(bool) {
		t.Error("Reloader should not be running initially")
	}
	
	// Start and test status
	if err := reloader.Start(); err != nil {
		t.Fatalf("Failed to start config reloader: %v", err)
	}
	defer reloader.Stop()
	
	status = reloader.GetStatus()
	if !status["running"].(bool) {
		t.Error("Reloader should be running after start")
	}
}
