package middleware

import (
	"context"
	"net/http"
	"time"
)

// Middleware defines the interface for HTTP middleware
type Middleware interface {
	// Handle wraps an HTTP handler with middleware functionality
	Handle(next http.Handler) http.Handler
	
	// Name returns the middleware name
	Name() string
	
	// Priority returns the middleware priority (lower number = higher priority)
	Priority() int
	
	// Configure configures the middleware
	Configure(config map[string]interface{}) error
	
	// Enabled returns whether the middleware is enabled
	Enabled() bool
	
	// SetEnabled sets the enabled state
	SetEnabled(enabled bool)
}

// Chain defines the interface for middleware chains
type Chain interface {
	// Add adds a middleware to the chain
	Add(middleware Middleware) error
	
	// Remove removes a middleware from the chain
	Remove(name string) error
	
	// Get gets a middleware by name
	Get(name string) (Middleware, error)
	
	// List lists all middlewares in the chain
	List() []Middleware
	
	// Build builds the middleware chain into an HTTP handler
	Build(final http.Handler) http.Handler
	
	// Clear clears all middlewares from the chain
	Clear()
	
	// Clone creates a copy of the chain
	Clone() Chain
}

// Factory defines the interface for middleware factories
type Factory interface {
	// Create creates a new middleware instance
	Create(name string, config map[string]interface{}) (Middleware, error)
	
	// Register registers a middleware type
	Register(name string, creator MiddlewareCreator) error
	
	// Unregister unregisters a middleware type
	Unregister(name string) error
	
	// List lists all registered middleware types
	List() []string
}

// MiddlewareCreator is a function that creates middleware instances
type MiddlewareCreator func(config map[string]interface{}) (Middleware, error)

// Manager defines the interface for middleware management
type Manager interface {
	// CreateChain creates a new middleware chain
	CreateChain(name string) (Chain, error)
	
	// GetChain gets a middleware chain by name
	GetChain(name string) (Chain, error)
	
	// RemoveChain removes a middleware chain
	RemoveChain(name string) error
	
	// ListChains lists all middleware chains
	ListChains() []string
	
	// GetFactory gets the middleware factory
	GetFactory() Factory
	
	// LoadConfig loads middleware configuration
	LoadConfig(config *Config) error
	
	// ReloadConfig reloads middleware configuration
	ReloadConfig() error
}

// Config represents middleware configuration
type Config struct {
	Chains map[string]*ChainConfig `json:"chains"`
	Global *ChainConfig            `json:"global,omitempty"`
}

// ChainConfig represents configuration for a middleware chain
type ChainConfig struct {
	Middlewares []*MiddlewareConfig `json:"middlewares"`
	Enabled     bool                `json:"enabled"`
}

// MiddlewareConfig represents configuration for a single middleware
type MiddlewareConfig struct {
	Name     string                 `json:"name"`
	Type     string                 `json:"type"`
	Priority int                    `json:"priority,omitempty"`
	Enabled  bool                   `json:"enabled"`
	Config   map[string]interface{} `json:"config,omitempty"`
}

// Context defines the interface for middleware context
type Context interface {
	// Request returns the HTTP request
	Request() *http.Request
	
	// ResponseWriter returns the HTTP response writer
	ResponseWriter() http.ResponseWriter
	
	// Set sets a value in the context
	Set(key string, value interface{})
	
	// Get gets a value from the context
	Get(key string) (interface{}, bool)
	
	// Delete deletes a value from the context
	Delete(key string)
	
	// Keys returns all keys in the context
	Keys() []string
	
	// Clone creates a copy of the context
	Clone() Context
}

// RateLimiter defines the interface for rate limiting middleware
type RateLimiter interface {
	Middleware
	
	// Allow checks if a request is allowed
	Allow(ctx context.Context, key string) (bool, error)
	
	// Remaining returns the number of remaining requests
	Remaining(ctx context.Context, key string) (int64, error)
	
	// Reset resets the rate limit for a key
	Reset(ctx context.Context, key string) error
	
	// SetLimit sets the rate limit
	SetLimit(limit int64, window time.Duration) error
}

// Cache defines the interface for caching middleware
type Cache interface {
	Middleware
	
	// Get gets a cached response
	Get(ctx context.Context, key string) (*CachedResponse, error)
	
	// Set caches a response
	Set(ctx context.Context, key string, response *CachedResponse, ttl time.Duration) error
	
	// Delete deletes a cached response
	Delete(ctx context.Context, key string) error
	
	// Clear clears all cached responses
	Clear(ctx context.Context) error
	
	// GenerateKey generates a cache key for a request
	GenerateKey(req *http.Request) string
}

// CachedResponse represents a cached HTTP response
type CachedResponse struct {
	StatusCode int                 `json:"status_code"`
	Headers    map[string][]string `json:"headers"`
	Body       []byte              `json:"body"`
	Timestamp  time.Time           `json:"timestamp"`
	TTL        time.Duration       `json:"ttl"`
}

// CORS defines the interface for CORS middleware
type CORS interface {
	Middleware
	
	// SetAllowedOrigins sets allowed origins
	SetAllowedOrigins(origins []string) error
	
	// SetAllowedMethods sets allowed methods
	SetAllowedMethods(methods []string) error
	
	// SetAllowedHeaders sets allowed headers
	SetAllowedHeaders(headers []string) error
	
	// SetExposedHeaders sets exposed headers
	SetExposedHeaders(headers []string) error
	
	// SetMaxAge sets the max age for preflight requests
	SetMaxAge(maxAge time.Duration) error
	
	// SetAllowCredentials sets whether credentials are allowed
	SetAllowCredentials(allow bool) error
}

// Compression defines the interface for compression middleware
type Compression interface {
	Middleware
	
	// SetLevel sets the compression level
	SetLevel(level int) error
	
	// SetMinSize sets the minimum size for compression
	SetMinSize(size int64) error
	
	// SetTypes sets the content types to compress
	SetTypes(types []string) error
	
	// AddType adds a content type to compress
	AddType(contentType string) error
	
	// RemoveType removes a content type from compression
	RemoveType(contentType string) error
}

// Logger defines the interface for logging middleware
type Logger interface {
	Middleware
	
	// SetFormat sets the log format
	SetFormat(format string) error
	
	// SetOutput sets the log output
	SetOutput(output string) error
	
	// SetLevel sets the log level
	SetLevel(level string) error
	
	// AddField adds a custom field to log entries
	AddField(name string, extractor FieldExtractor) error
	
	// RemoveField removes a custom field
	RemoveField(name string) error
}

// FieldExtractor extracts a field value from the request/response
type FieldExtractor func(req *http.Request, resp http.ResponseWriter) interface{}

// Metrics defines the interface for metrics middleware
type Metrics interface {
	Middleware
	
	// RecordRequest records request metrics
	RecordRequest(req *http.Request, statusCode int, duration time.Duration, size int64)
	
	// GetMetrics gets current metrics
	GetMetrics() (*MetricsData, error)
	
	// ResetMetrics resets all metrics
	ResetMetrics() error
	
	// SetLabels sets custom labels for metrics
	SetLabels(labels map[string]string) error
}

// MetricsData represents metrics data
type MetricsData struct {
	RequestCount    int64                    `json:"request_count"`
	ErrorCount      int64                    `json:"error_count"`
	ResponseTime    time.Duration            `json:"response_time"`
	AvgResponseTime time.Duration            `json:"avg_response_time"`
	RequestRate     float64                  `json:"request_rate"`
	ErrorRate       float64                  `json:"error_rate"`
	StatusCodes     map[int]int64            `json:"status_codes"`
	Methods         map[string]int64         `json:"methods"`
	Paths           map[string]int64         `json:"paths"`
	CustomMetrics   map[string]interface{}   `json:"custom_metrics,omitempty"`
}

// Transform defines the interface for request/response transformation middleware
type Transform interface {
	Middleware
	
	// AddRequestTransform adds a request transformation
	AddRequestTransform(transform RequestTransform) error
	
	// AddResponseTransform adds a response transformation
	AddResponseTransform(transform ResponseTransform) error
	
	// RemoveRequestTransform removes a request transformation
	RemoveRequestTransform(name string) error
	
	// RemoveResponseTransform removes a response transformation
	RemoveResponseTransform(name string) error
}

// RequestTransform defines a request transformation
type RequestTransform interface {
	// Name returns the transform name
	Name() string
	
	// Transform transforms the request
	Transform(req *http.Request) error
}

// ResponseTransform defines a response transformation
type ResponseTransform interface {
	// Name returns the transform name
	Name() string
	
	// Transform transforms the response
	Transform(resp http.ResponseWriter, body []byte) ([]byte, error)
}
