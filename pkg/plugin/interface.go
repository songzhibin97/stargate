package plugin

import (
	"context"
	"net/http"
)

// Plugin represents the core plugin interface
type Plugin interface {
	// Name returns the plugin name
	Name() string
	
	// Version returns the plugin version
	Version() string
	
	// Description returns the plugin description
	Description() string
	
	// Initialize initializes the plugin with configuration
	Initialize(config map[string]interface{}) error
	
	// Handle wraps an HTTP handler with plugin functionality
	Handle(next http.Handler) http.Handler
	
	// Cleanup performs cleanup when the plugin is unloaded
	Cleanup() error
}

// AuthPlugin represents an authentication plugin
type AuthPlugin interface {
	Plugin
	
	// Authenticate authenticates a request
	Authenticate(r *http.Request) (*AuthResult, error)
	
	// GetUserInfo returns user information from the request
	GetUserInfo(r *http.Request) (*UserInfo, error)
}

// RateLimitPlugin represents a rate limiting plugin
type RateLimitPlugin interface {
	Plugin
	
	// IsAllowed checks if a request is allowed
	IsAllowed(r *http.Request) (*RateLimitResult, error)
	
	// GetQuota returns the current quota for a request
	GetQuota(r *http.Request) (*Quota, error)
}

// TransformPlugin represents a request/response transformation plugin
type TransformPlugin interface {
	Plugin
	
	// TransformRequest transforms the request
	TransformRequest(r *http.Request) (*http.Request, error)
	
	// TransformResponse transforms the response
	TransformResponse(w http.ResponseWriter, r *http.Request) error
}

// LoggingPlugin represents a logging plugin
type LoggingPlugin interface {
	Plugin
	
	// LogRequest logs request information
	LogRequest(r *http.Request, metadata map[string]interface{}) error
	
	// LogResponse logs response information
	LogResponse(r *http.Request, statusCode int, responseSize int64, duration int64) error
}

// MetricsPlugin represents a metrics collection plugin
type MetricsPlugin interface {
	Plugin
	
	// RecordMetric records a metric
	RecordMetric(name string, value float64, labels map[string]string) error
	
	// IncrementCounter increments a counter metric
	IncrementCounter(name string, labels map[string]string) error
	
	// RecordHistogram records a histogram metric
	RecordHistogram(name string, value float64, labels map[string]string) error
}

// CircuitBreakerPlugin represents a circuit breaker plugin
type CircuitBreakerPlugin interface {
	Plugin
	
	// IsCircuitOpen checks if the circuit is open for a request
	IsCircuitOpen(r *http.Request) (bool, error)
	
	// RecordSuccess records a successful request
	RecordSuccess(r *http.Request) error
	
	// RecordFailure records a failed request
	RecordFailure(r *http.Request, err error) error
}

// CachePlugin represents a caching plugin
type CachePlugin interface {
	Plugin
	
	// Get retrieves a cached response
	Get(key string) (*CachedResponse, error)
	
	// Set stores a response in cache
	Set(key string, response *CachedResponse, ttl int64) error
	
	// Delete removes a cached response
	Delete(key string) error
	
	// GenerateKey generates a cache key for a request
	GenerateKey(r *http.Request) string
}

// HealthCheckPlugin represents a health check plugin
type HealthCheckPlugin interface {
	Plugin
	
	// CheckHealth performs a health check
	CheckHealth(ctx context.Context, target string) (*HealthResult, error)
	
	// GetHealthStatus returns the current health status
	GetHealthStatus(target string) (*HealthResult, error)
}

// AuthResult represents authentication result
type AuthResult struct {
	Authenticated bool                   `json:"authenticated"`
	UserID        string                 `json:"user_id,omitempty"`
	Username      string                 `json:"username,omitempty"`
	Roles         []string               `json:"roles,omitempty"`
	Permissions   []string               `json:"permissions,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Error         string                 `json:"error,omitempty"`
}

// UserInfo represents user information
type UserInfo struct {
	ID          string                 `json:"id"`
	Username    string                 `json:"username"`
	Email       string                 `json:"email,omitempty"`
	DisplayName string                 `json:"display_name,omitempty"`
	Roles       []string               `json:"roles,omitempty"`
	Permissions []string               `json:"permissions,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// RateLimitResult represents rate limiting result
type RateLimitResult struct {
	Allowed       bool  `json:"allowed"`
	Remaining     int64 `json:"remaining"`
	ResetTime     int64 `json:"reset_time"`
	RetryAfter    int64 `json:"retry_after,omitempty"`
	Error         string `json:"error,omitempty"`
}

// Quota represents rate limiting quota
type Quota struct {
	Limit     int64 `json:"limit"`
	Remaining int64 `json:"remaining"`
	ResetTime int64 `json:"reset_time"`
	Window    int64 `json:"window"`
}

// CachedResponse represents a cached HTTP response
type CachedResponse struct {
	StatusCode int                 `json:"status_code"`
	Headers    map[string][]string `json:"headers"`
	Body       []byte              `json:"body"`
	CreatedAt  int64               `json:"created_at"`
	ExpiresAt  int64               `json:"expires_at"`
}

// HealthResult represents health check result
type HealthResult struct {
	Healthy   bool                   `json:"healthy"`
	Status    string                 `json:"status"`
	Message   string                 `json:"message,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CheckedAt int64                  `json:"checked_at"`
}

// Registry manages plugin registration and discovery
type Registry interface {
	// Register registers a plugin
	Register(plugin Plugin) error
	
	// Unregister unregisters a plugin
	Unregister(name string) error
	
	// Get retrieves a plugin by name
	Get(name string) (Plugin, error)
	
	// List returns all registered plugins
	List() []Plugin
	
	// ListByType returns plugins of a specific type
	ListByType(pluginType string) []Plugin
}

// Manager manages plugin lifecycle
type Manager interface {
	// Load loads a plugin from file
	Load(path string) (Plugin, error)
	
	// Unload unloads a plugin
	Unload(name string) error
	
	// Enable enables a plugin
	Enable(name string) error
	
	// Disable disables a plugin
	Disable(name string) error
	
	// Configure configures a plugin
	Configure(name string, config map[string]interface{}) error
	
	// GetStatus returns plugin status
	GetStatus(name string) (*PluginStatus, error)
	
	// ListPlugins returns all managed plugins
	ListPlugins() []*PluginInfo
}

// PluginInfo represents plugin information
type PluginInfo struct {
	Name        string                 `json:"name"`
	Version     string                 `json:"version"`
	Description string                 `json:"description"`
	Type        string                 `json:"type"`
	Enabled     bool                   `json:"enabled"`
	Config      map[string]interface{} `json:"config,omitempty"`
	LoadedAt    int64                  `json:"loaded_at"`
}

// PluginStatus represents plugin status
type PluginStatus struct {
	Name      string                 `json:"name"`
	Enabled   bool                   `json:"enabled"`
	Healthy   bool                   `json:"healthy"`
	Error     string                 `json:"error,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	UpdatedAt int64                  `json:"updated_at"`
}

// Context keys for plugin data
type ContextKey string

const (
	// ContextKeyAuthResult stores authentication result
	ContextKeyAuthResult ContextKey = "auth_result"
	
	// ContextKeyUserInfo stores user information
	ContextKeyUserInfo ContextKey = "user_info"
	
	// ContextKeyRateLimit stores rate limit information
	ContextKeyRateLimit ContextKey = "rate_limit"
	
	// ContextKeyMetrics stores metrics data
	ContextKeyMetrics ContextKey = "metrics"
	
	// ContextKeyPluginData stores generic plugin data
	ContextKeyPluginData ContextKey = "plugin_data"
)

// Helper functions for context manipulation
func SetAuthResult(r *http.Request, result *AuthResult) *http.Request {
	ctx := context.WithValue(r.Context(), ContextKeyAuthResult, result)
	return r.WithContext(ctx)
}

func GetAuthResult(r *http.Request) (*AuthResult, bool) {
	result, ok := r.Context().Value(ContextKeyAuthResult).(*AuthResult)
	return result, ok
}

func SetUserInfo(r *http.Request, userInfo *UserInfo) *http.Request {
	ctx := context.WithValue(r.Context(), ContextKeyUserInfo, userInfo)
	return r.WithContext(ctx)
}

func GetUserInfo(r *http.Request) (*UserInfo, bool) {
	userInfo, ok := r.Context().Value(ContextKeyUserInfo).(*UserInfo)
	return userInfo, ok
}

func SetRateLimitResult(r *http.Request, result *RateLimitResult) *http.Request {
	ctx := context.WithValue(r.Context(), ContextKeyRateLimit, result)
	return r.WithContext(ctx)
}

func GetRateLimitResult(r *http.Request) (*RateLimitResult, bool) {
	result, ok := r.Context().Value(ContextKeyRateLimit).(*RateLimitResult)
	return result, ok
}

func SetPluginData(r *http.Request, key string, data interface{}) *http.Request {
	pluginData, _ := r.Context().Value(ContextKeyPluginData).(map[string]interface{})
	if pluginData == nil {
		pluginData = make(map[string]interface{})
	}
	pluginData[key] = data
	ctx := context.WithValue(r.Context(), ContextKeyPluginData, pluginData)
	return r.WithContext(ctx)
}

func GetPluginData(r *http.Request, key string) (interface{}, bool) {
	pluginData, ok := r.Context().Value(ContextKeyPluginData).(map[string]interface{})
	if !ok {
		return nil, false
	}
	data, exists := pluginData[key]
	return data, exists
}
