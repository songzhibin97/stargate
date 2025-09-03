package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/controller/api"
	"github.com/songzhibin97/stargate/internal/store"
)

// Manager manages middleware chain and provides hot reload capabilities
type Manager struct {
	config     *config.Config
	store      store.Store
	mu         sync.RWMutex
	middlewares map[string]MiddlewareFactory
	chain      []http.Handler
	plugins    map[string]*api.Plugin
}

// MiddlewareFactory creates middleware instances
type MiddlewareFactory func(config map[string]interface{}) (http.Handler, error)

// NewManager creates a new middleware manager
func NewManager(cfg *config.Config, store store.Store) *Manager {
	return &Manager{
		config:      cfg,
		store:       store,
		middlewares: make(map[string]MiddlewareFactory),
		plugins:     make(map[string]*api.Plugin),
	}
}

// RegisterMiddleware registers a middleware factory
func (m *Manager) RegisterMiddleware(pluginType string, factory MiddlewareFactory) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.middlewares[pluginType] = factory
	log.Printf("Registered middleware factory: %s", pluginType)
}

// BuildChain builds the middleware chain from plugin configurations
func (m *Manager) BuildChain() ([]http.Handler, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Load all plugin configurations
	pluginsData, err := m.store.List(ctx, "plugins/")
	if err != nil {
		return nil, fmt.Errorf("failed to list plugins: %w", err)
	}

	// Parse plugins
	var plugins []*api.Plugin
	for _, data := range pluginsData {
		var plugin api.Plugin
		if err := json.Unmarshal(data, &plugin); err != nil {
			log.Printf("Failed to unmarshal plugin: %v", err)
			continue
		}
		if plugin.Enabled {
			plugins = append(plugins, &plugin)
		}
	}

	// Sort plugins by priority (higher priority first)
	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].Priority > plugins[j].Priority
	})

	// Build middleware chain
	var chain []http.Handler
	for _, plugin := range plugins {
		factory, exists := m.middlewares[plugin.Type]
		if !exists {
			log.Printf("No factory found for plugin type: %s", plugin.Type)
			continue
		}

		middleware, err := factory(plugin.Config)
		if err != nil {
			log.Printf("Failed to create middleware for plugin %s: %v", plugin.ID, err)
			continue
		}

		chain = append(chain, middleware)
		log.Printf("Added middleware: %s (priority: %d)", plugin.Name, plugin.Priority)
	}

	m.chain = chain
	m.updatePluginsCache(plugins)

	log.Printf("Built middleware chain with %d middlewares", len(chain))
	return chain, nil
}

// RebuildChain rebuilds the entire middleware chain
func (m *Manager) RebuildChain() error {
	_, err := m.BuildChain()
	return err
}

// GetChain returns the current middleware chain
func (m *Manager) GetChain() []http.Handler {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to prevent external modification
	chain := make([]http.Handler, len(m.chain))
	copy(chain, m.chain)
	return chain
}

// updatePluginsCache updates the internal plugins cache
func (m *Manager) updatePluginsCache(plugins []*api.Plugin) {
	m.plugins = make(map[string]*api.Plugin)
	for _, plugin := range plugins {
		m.plugins[plugin.ID] = plugin
	}
}

// GetPlugin returns a plugin by ID
func (m *Manager) GetPlugin(pluginID string) (*api.Plugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plugin, exists := m.plugins[pluginID]
	return plugin, exists
}

// ListPlugins returns all cached plugins
func (m *Manager) ListPlugins() []*api.Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plugins := make([]*api.Plugin, 0, len(m.plugins))
	for _, plugin := range m.plugins {
		plugins = append(plugins, plugin)
	}

	return plugins
}

// InitializeBuiltinMiddlewares initializes built-in middleware factories
func (m *Manager) InitializeBuiltinMiddlewares() {
	// Rate Limiting
	m.RegisterMiddleware("rate_limit", func(config map[string]interface{}) (http.Handler, error) {
		return m.createRateLimitMiddleware(config)
	})

	// CORS
	m.RegisterMiddleware("cors", func(config map[string]interface{}) (http.Handler, error) {
		return m.createCORSMiddleware(config)
	})

	// Authentication
	m.RegisterMiddleware("auth", func(config map[string]interface{}) (http.Handler, error) {
		return m.createAuthMiddleware(config)
	})

	// Circuit Breaker
	m.RegisterMiddleware("circuit_breaker", func(config map[string]interface{}) (http.Handler, error) {
		return m.createCircuitBreakerMiddleware(config)
	})

	// Traffic Mirror
	m.RegisterMiddleware("traffic_mirror", func(config map[string]interface{}) (http.Handler, error) {
		return m.createTrafficMirrorMiddleware(config)
	})

	// Header Transform
	m.RegisterMiddleware("header_transform", func(config map[string]interface{}) (http.Handler, error) {
		return m.createHeaderTransformMiddleware(config)
	})

	// Mock Response
	m.RegisterMiddleware("mock_response", func(config map[string]interface{}) (http.Handler, error) {
		return m.createMockResponseMiddleware(config)
	})

	log.Println("Built-in middleware factories initialized")
}

// createRateLimitMiddleware creates a rate limiting middleware
func (m *Manager) createRateLimitMiddleware(config map[string]interface{}) (http.Handler, error) {
	// Extract configuration
	maxRequests, ok := config["max_requests"].(float64)
	if !ok {
		maxRequests = 100 // default
	}

	windowSize, ok := config["window_size"].(string)
	if !ok {
		windowSize = "1m" // default
	}

	// Create and return rate limit middleware
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Rate limiting logic would go here
		// For now, just pass through
		log.Printf("Rate limit middleware: max_requests=%v, window_size=%s", maxRequests, windowSize)
	}), nil
}

// createCORSMiddleware creates a CORS middleware
func (m *Manager) createCORSMiddleware(config map[string]interface{}) (http.Handler, error) {
	// Extract configuration
	allowOrigins, ok := config["allow_origins"].([]interface{})
	if !ok {
		allowOrigins = []interface{}{"*"} // default
	}

	allowMethods, ok := config["allow_methods"].([]interface{})
	if !ok {
		allowMethods = []interface{}{"GET", "POST", "PUT", "DELETE", "OPTIONS"} // default
	}

	// Create and return CORS middleware
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS logic would go here
		log.Printf("CORS middleware: origins=%v, methods=%v", allowOrigins, allowMethods)
	}), nil
}

// createAuthMiddleware creates an authentication middleware
func (m *Manager) createAuthMiddleware(config map[string]interface{}) (http.Handler, error) {
	// Extract configuration
	authType, ok := config["type"].(string)
	if !ok {
		authType = "jwt" // default
	}

	// Create and return auth middleware
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Authentication logic would go here
		log.Printf("Auth middleware: type=%s", authType)
	}), nil
}

// createCircuitBreakerMiddleware creates a circuit breaker middleware
func (m *Manager) createCircuitBreakerMiddleware(config map[string]interface{}) (http.Handler, error) {
	// Extract configuration
	failureThreshold, ok := config["failure_threshold"].(float64)
	if !ok {
		failureThreshold = 5 // default
	}

	// Create and return circuit breaker middleware
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Circuit breaker logic would go here
		log.Printf("Circuit breaker middleware: threshold=%v", failureThreshold)
	}), nil
}

// createTrafficMirrorMiddleware creates a traffic mirror middleware
func (m *Manager) createTrafficMirrorMiddleware(config map[string]interface{}) (http.Handler, error) {
	// Extract configuration
	mirrorURL, ok := config["mirror_url"].(string)
	if !ok {
		return nil, fmt.Errorf("mirror_url is required for traffic mirror middleware")
	}

	// Create and return traffic mirror middleware
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Traffic mirror logic would go here
		log.Printf("Traffic mirror middleware: mirror_url=%s", mirrorURL)
	}), nil
}

// createHeaderTransformMiddleware creates a header transform middleware
func (m *Manager) createHeaderTransformMiddleware(config map[string]interface{}) (http.Handler, error) {
	// Extract configuration
	addHeaders, ok := config["add_headers"].(map[string]interface{})
	if !ok {
		addHeaders = make(map[string]interface{})
	}

	// Create and return header transform middleware
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Header transform logic would go here
		log.Printf("Header transform middleware: add_headers=%v", addHeaders)
	}), nil
}

// createMockResponseMiddleware creates a mock response middleware
func (m *Manager) createMockResponseMiddleware(config map[string]interface{}) (http.Handler, error) {
	// Extract configuration
	statusCode, ok := config["status_code"].(float64)
	if !ok {
		statusCode = 200 // default
	}

	body, ok := config["body"].(string)
	if !ok {
		body = `{"message": "mock response"}` // default
	}

	// Create and return mock response middleware
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock response logic would go here
		log.Printf("Mock response middleware: status=%v, body=%s", statusCode, body)
	}), nil
}

// Health returns the health status of the middleware manager
func (m *Manager) Health() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"middlewares_count": len(m.middlewares),
		"chain_length":      len(m.chain),
		"plugins_count":     len(m.plugins),
	}
}
