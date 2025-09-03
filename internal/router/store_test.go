package router

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	pkgConfig "github.com/songzhibin97/stargate/pkg/config"
)

// MockConfigSource implements config.Source interface for testing
type MockConfigSource struct {
	data     []byte
	watchers []chan []byte
	mu       sync.RWMutex
	closed   bool
}

func NewMockConfigSource(initialData []byte) *MockConfigSource {
	return &MockConfigSource{
		data:     initialData,
		watchers: make([]chan []byte, 0),
	}
}

func (m *MockConfigSource) Get() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if m.closed {
		return nil, fmt.Errorf("source is closed")
	}
	
	return m.data, nil
}

func (m *MockConfigSource) Watch(ctx context.Context) (<-chan []byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil, fmt.Errorf("source is closed")
	}

	ch := make(chan []byte, 1)
	m.watchers = append(m.watchers, ch)

	// Send initial data immediately in the same goroutine
	select {
	case ch <- m.data:
	default:
		// Channel is full, skip
	}

	// Handle context cancellation in a separate goroutine
	go func() {
		<-ctx.Done()

		m.mu.Lock()
		defer m.mu.Unlock()
		// Remove from watchers and close
		for i, watcher := range m.watchers {
			if watcher == ch {
				m.watchers = append(m.watchers[:i], m.watchers[i+1:]...)
				close(ch)
				break
			}
		}
	}()

	return ch, nil
}

func (m *MockConfigSource) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	m.closed = true

	// Close all watchers safely
	for _, ch := range m.watchers {
		select {
		case <-ch:
			// Channel already closed
		default:
			close(ch)
		}
	}
	m.watchers = nil

	return nil
}

// UpdateData simulates a configuration update
func (m *MockConfigSource) UpdateData(newData []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.closed {
		return
	}
	
	m.data = newData
	
	// Notify all watchers
	for _, ch := range m.watchers {
		select {
		case ch <- newData:
		default:
			// Channel is full or closed, skip
		}
	}
}

func TestNewStore(t *testing.T) {
	initialConfig := `
routes:
  - id: "test-route"
    name: "Test Route"
    rules:
      hosts: ["example.com"]
      paths:
        - type: "prefix"
          value: "/api"
    upstream_id: "test-upstream"

upstreams:
  - id: "test-upstream"
    name: "Test Upstream"
    targets:
      - url: "http://backend.example.com"
`

	tests := []struct {
		name        string
		source      pkgConfig.Source
		engine      *Engine
		expectError bool
	}{
		{
			name:        "nil source",
			source:      nil,
			engine:      NewEngine(&config.Config{}),
			expectError: true,
		},
		{
			name:        "nil engine",
			source:      NewMockConfigSource([]byte(initialConfig)),
			engine:      nil,
			expectError: true,
		},
		{
			name:        "valid parameters",
			source:      NewMockConfigSource([]byte(initialConfig)),
			engine:      NewEngine(&config.Config{}),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := NewStore(tt.source, tt.engine)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if store == nil {
				t.Errorf("Expected store but got nil")
			}
			
			// Clean up
			if store != nil && tt.source != nil {
				tt.source.Close()
			}
		})
	}
}

func TestStore_StartStop(t *testing.T) {
	initialConfig := `
routes:
  - id: "test-route"
    name: "Test Route"
    rules:
      hosts: ["example.com"]
      paths:
        - type: "prefix"
          value: "/api"
    upstream_id: "test-upstream"

upstreams:
  - id: "test-upstream"
    name: "Test Upstream"
    targets:
      - url: "http://backend.example.com"
`

	source := NewMockConfigSource([]byte(initialConfig))
	engine := NewEngine(&config.Config{})
	
	store, err := NewStore(source, engine)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	ctx := context.Background()

	// Test Start
	err = store.Start(ctx)
	if err != nil {
		t.Errorf("Failed to start store: %v", err)
	}

	if !store.IsRunning() {
		t.Errorf("Expected store to be running")
	}

	// Test starting already running store
	err = store.Start(ctx)
	if err == nil {
		t.Errorf("Expected error when starting already running store")
	}

	// Test Stop
	err = store.Stop()
	if err != nil {
		t.Errorf("Failed to stop store: %v", err)
	}

	if store.IsRunning() {
		t.Errorf("Expected store to be stopped")
	}

	// Test stopping already stopped store
	err = store.Stop()
	if err != nil {
		t.Errorf("Unexpected error when stopping already stopped store: %v", err)
	}
}

func TestStore_Reload(t *testing.T) {
	initialConfig := `
routes:
  - id: "test-route"
    name: "Test Route"
    rules:
      hosts: ["example.com"]
      paths:
        - type: "prefix"
          value: "/api"
    upstream_id: "test-upstream"

upstreams:
  - id: "test-upstream"
    name: "Test Upstream"
    targets:
      - url: "http://backend.example.com"
`

	source := NewMockConfigSource([]byte(initialConfig))
	engine := NewEngine(&config.Config{})
	
	store, err := NewStore(source, engine)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	ctx := context.Background()

	// Test reload before start (should fail)
	err = store.Reload()
	if err == nil {
		t.Errorf("Expected error when reloading stopped store")
	}

	// Start the store
	err = store.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start store: %v", err)
	}
	defer store.Stop()

	// Test successful reload
	err = store.Reload()
	if err != nil {
		t.Errorf("Failed to reload store: %v", err)
	}

	// Verify last update time was updated
	lastUpdate := store.GetLastUpdate()
	if lastUpdate.IsZero() {
		t.Errorf("Expected last update time to be set")
	}
}

func TestStore_ConfigurationUpdate(t *testing.T) {
	initialConfig := `
routes:
  - id: "test-route"
    name: "Test Route"
    rules:
      hosts: ["example.com"]
      paths:
        - type: "prefix"
          value: "/api"
    upstream_id: "test-upstream"

upstreams:
  - id: "test-upstream"
    name: "Test Upstream"
    targets:
      - url: "http://backend.example.com"
`

	updatedConfig := `
routes:
  - id: "test-route"
    name: "Updated Test Route"
    rules:
      hosts: ["example.com", "api.example.com"]
      paths:
        - type: "prefix"
          value: "/api/v1"
    upstream_id: "test-upstream"

upstreams:
  - id: "test-upstream"
    name: "Test Upstream"
    targets:
      - url: "http://backend.example.com"
      - url: "http://backend2.example.com"
`

	source := NewMockConfigSource([]byte(initialConfig))
	engine := NewEngine(&config.Config{})
	
	store, err := NewStore(source, engine)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	ctx := context.Background()

	// Start the store
	err = store.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start store: %v", err)
	}
	defer store.Stop()

	// Wait a bit for initial configuration to be loaded
	time.Sleep(100 * time.Millisecond)

	initialUpdate := store.GetLastUpdate()

	// Update configuration
	source.UpdateData([]byte(updatedConfig))

	// Wait for update to be processed
	time.Sleep(200 * time.Millisecond)

	// Verify that last update time changed
	finalUpdate := store.GetLastUpdate()
	if !finalUpdate.After(initialUpdate) {
		t.Errorf("Expected last update time to be updated after configuration change")
	}

	// Verify configuration was updated
	config := store.GetConfigManager().GetConfig()
	if len(config.Routes) != 1 {
		t.Errorf("Expected 1 route, got %d", len(config.Routes))
	}

	if config.Routes[0].Name != "Updated Test Route" {
		t.Errorf("Expected route name to be updated, got %s", config.Routes[0].Name)
	}
}
