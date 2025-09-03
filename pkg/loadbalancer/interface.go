package loadbalancer

import (
	"context"
	"net/http"
	"time"
)

// LoadBalancer defines the interface for load balancing
type LoadBalancer interface {
	// Select selects a backend server for the request
	Select(ctx context.Context, req *http.Request) (*Backend, error)
	
	// Name returns the load balancer name
	Name() string
	
	// Algorithm returns the load balancing algorithm
	Algorithm() string
	
	// AddBackend adds a backend server
	AddBackend(backend *Backend) error
	
	// RemoveBackend removes a backend server
	RemoveBackend(id string) error
	
	// UpdateBackend updates a backend server
	UpdateBackend(backend *Backend) error
	
	// ListBackends lists all backend servers
	ListBackends() []*Backend
	
	// GetBackend gets a backend server by ID
	GetBackend(id string) (*Backend, error)
	
	// SetHealthChecker sets the health checker
	SetHealthChecker(checker HealthChecker) error
	
	// Start starts the load balancer
	Start(ctx context.Context) error
	
	// Stop stops the load balancer
	Stop() error
}

// Backend represents a backend server
type Backend struct {
	ID       string            `json:"id"`
	Address  string            `json:"address"`
	Port     int               `json:"port"`
	Weight   int               `json:"weight"`
	Priority int               `json:"priority"`
	Tags     map[string]string `json:"tags,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Status   BackendStatus     `json:"status"`
	Health   *HealthStatus     `json:"health,omitempty"`
	Stats    *BackendStats     `json:"stats,omitempty"`
	Config   *BackendConfig    `json:"config,omitempty"`
}

// BackendStatus represents the status of a backend
type BackendStatus string

const (
	BackendStatusHealthy   BackendStatus = "healthy"
	BackendStatusUnhealthy BackendStatus = "unhealthy"
	BackendStatusDraining  BackendStatus = "draining"
	BackendStatusDisabled  BackendStatus = "disabled"
)

// HealthStatus represents the health status of a backend
type HealthStatus struct {
	Status      BackendStatus `json:"status"`
	Message     string        `json:"message,omitempty"`
	LastCheck   time.Time     `json:"last_check"`
	CheckCount  int64         `json:"check_count"`
	FailCount   int64         `json:"fail_count"`
	SuccessRate float64       `json:"success_rate"`
}

// BackendStats represents statistics for a backend
type BackendStats struct {
	RequestCount    int64         `json:"request_count"`
	ErrorCount      int64         `json:"error_count"`
	ResponseTime    time.Duration `json:"response_time"`
	AvgResponseTime time.Duration `json:"avg_response_time"`
	LastRequestTime time.Time     `json:"last_request_time"`
	BytesSent       int64         `json:"bytes_sent"`
	BytesReceived   int64         `json:"bytes_received"`
}

// BackendConfig represents configuration for a backend
type BackendConfig struct {
	MaxConnections int           `json:"max_connections,omitempty"`
	Timeout        time.Duration `json:"timeout,omitempty"`
	RetryCount     int           `json:"retry_count,omitempty"`
	RetryTimeout   time.Duration `json:"retry_timeout,omitempty"`
	KeepAlive      bool          `json:"keep_alive,omitempty"`
}

// HealthChecker defines the interface for health checking
type HealthChecker interface {
	// Check performs a health check on a backend
	Check(ctx context.Context, backend *Backend) (*HealthStatus, error)
	
	// Start starts the health checker
	Start(ctx context.Context) error
	
	// Stop stops the health checker
	Stop() error
	
	// Configure configures the health checker
	Configure(config *HealthCheckConfig) error
	
	// Subscribe subscribes to health status changes
	Subscribe(callback HealthChangeCallback) error
}

// HealthCheckConfig represents health check configuration
type HealthCheckConfig struct {
	Interval          time.Duration `json:"interval"`
	Timeout           time.Duration `json:"timeout"`
	HealthyThreshold  int           `json:"healthy_threshold"`
	UnhealthyThreshold int          `json:"unhealthy_threshold"`
	Path              string        `json:"path,omitempty"`
	Method            string        `json:"method,omitempty"`
	Headers           map[string]string `json:"headers,omitempty"`
	ExpectedStatus    []int         `json:"expected_status,omitempty"`
	ExpectedBody      string        `json:"expected_body,omitempty"`
}

// HealthChangeCallback is called when backend health status changes
type HealthChangeCallback func(backend *Backend, oldStatus, newStatus BackendStatus)

// Algorithm defines the interface for load balancing algorithms
type Algorithm interface {
	// Select selects a backend from the available backends
	Select(ctx context.Context, backends []*Backend, req *http.Request) (*Backend, error)
	
	// Name returns the algorithm name
	Name() string
	
	// Configure configures the algorithm
	Configure(config map[string]interface{}) error
}

// StickySession defines the interface for session affinity
type StickySession interface {
	// GetBackend gets the backend for a session
	GetBackend(ctx context.Context, sessionID string) (*Backend, error)
	
	// SetBackend sets the backend for a session
	SetBackend(ctx context.Context, sessionID string, backend *Backend) error
	
	// RemoveSession removes a session
	RemoveSession(ctx context.Context, sessionID string) error
	
	// ExtractSessionID extracts session ID from request
	ExtractSessionID(req *http.Request) (string, error)
	
	// Configure configures the sticky session
	Configure(config *StickySessionConfig) error
}

// StickySessionConfig represents sticky session configuration
type StickySessionConfig struct {
	CookieName   string        `json:"cookie_name,omitempty"`
	HeaderName   string        `json:"header_name,omitempty"`
	TTL          time.Duration `json:"ttl,omitempty"`
	Secure       bool          `json:"secure,omitempty"`
	HttpOnly     bool          `json:"http_only,omitempty"`
	SameSite     string        `json:"same_site,omitempty"`
}

// CircuitBreaker defines the interface for circuit breaker functionality
type CircuitBreaker interface {
	// Execute executes a function with circuit breaker protection
	Execute(ctx context.Context, fn func() error) error
	
	// State returns the current state of the circuit breaker
	State() CircuitBreakerState
	
	// Reset resets the circuit breaker
	Reset()
	
	// Configure configures the circuit breaker
	Configure(config *CircuitBreakerConfig) error
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState string

const (
	CircuitBreakerStateClosed   CircuitBreakerState = "closed"
	CircuitBreakerStateOpen     CircuitBreakerState = "open"
	CircuitBreakerStateHalfOpen CircuitBreakerState = "half_open"
)

// CircuitBreakerConfig represents circuit breaker configuration
type CircuitBreakerConfig struct {
	FailureThreshold int           `json:"failure_threshold"`
	RecoveryTimeout  time.Duration `json:"recovery_timeout"`
	SuccessThreshold int           `json:"success_threshold"`
	Timeout          time.Duration `json:"timeout"`
}

// Manager defines the interface for load balancer management
type Manager interface {
	// CreateLoadBalancer creates a new load balancer
	CreateLoadBalancer(name string, config *Config) (LoadBalancer, error)
	
	// GetLoadBalancer gets a load balancer by name
	GetLoadBalancer(name string) (LoadBalancer, error)
	
	// RemoveLoadBalancer removes a load balancer
	RemoveLoadBalancer(name string) error
	
	// ListLoadBalancers lists all load balancers
	ListLoadBalancers() []string
	
	// RegisterAlgorithm registers a load balancing algorithm
	RegisterAlgorithm(name string, algorithm Algorithm) error
	
	// GetAlgorithm gets an algorithm by name
	GetAlgorithm(name string) (Algorithm, error)
	
	// ListAlgorithms lists all registered algorithms
	ListAlgorithms() []string
}

// Config represents load balancer configuration
type Config struct {
	Algorithm       string                 `json:"algorithm"`
	Backends        []*Backend             `json:"backends"`
	HealthCheck     *HealthCheckConfig     `json:"health_check,omitempty"`
	StickySession   *StickySessionConfig   `json:"sticky_session,omitempty"`
	CircuitBreaker  *CircuitBreakerConfig  `json:"circuit_breaker,omitempty"`
	Options         map[string]interface{} `json:"options,omitempty"`
}

// Metrics defines the interface for load balancer metrics
type Metrics interface {
	// RecordRequest records a request metric
	RecordRequest(backend *Backend, duration time.Duration, success bool)
	
	// GetStats gets statistics for a backend
	GetStats(backendID string) (*BackendStats, error)
	
	// GetOverallStats gets overall statistics
	GetOverallStats() (*OverallStats, error)
	
	// Reset resets all metrics
	Reset()
}

// OverallStats represents overall load balancer statistics
type OverallStats struct {
	TotalRequests   int64         `json:"total_requests"`
	TotalErrors     int64         `json:"total_errors"`
	AvgResponseTime time.Duration `json:"avg_response_time"`
	RequestRate     float64       `json:"request_rate"`
	ErrorRate       float64       `json:"error_rate"`
	ActiveBackends  int           `json:"active_backends"`
	TotalBackends   int           `json:"total_backends"`
}
