package router

import (
	"context"
	"net/http"
	"time"
)

// Router defines the interface for HTTP routing
type Router interface {
	// Match matches a request to a route
	Match(req *http.Request) (*Route, error)
	
	// AddRoute adds a new route
	AddRoute(route *Route) error
	
	// RemoveRoute removes a route by ID
	RemoveRoute(id string) error
	
	// UpdateRoute updates an existing route
	UpdateRoute(route *Route) error
	
	// GetRoute gets a route by ID
	GetRoute(id string) (*Route, error)
	
	// ListRoutes lists all routes
	ListRoutes() []*Route
	
	// Reload reloads the routing configuration
	Reload() error
	
	// Start starts the router
	Start(ctx context.Context) error
	
	// Stop stops the router
	Stop() error
}

// Route represents a routing rule
type Route struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Priority    int               `json:"priority"`
	Enabled     bool              `json:"enabled"`
	Methods     []string          `json:"methods,omitempty"`
	Hosts       []string          `json:"hosts,omitempty"`
	Paths       []string          `json:"paths,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Query       map[string]string `json:"query,omitempty"`
	Upstream    *Upstream         `json:"upstream"`
	Middleware  []string          `json:"middleware,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// Upstream represents an upstream service
type Upstream struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Scheme          string            `json:"scheme"`
	LoadBalancer    string            `json:"load_balancer"`
	Servers         []*Server         `json:"servers"`
	HealthCheck     *HealthCheck      `json:"health_check,omitempty"`
	CircuitBreaker  *CircuitBreaker   `json:"circuit_breaker,omitempty"`
	Retry           *RetryConfig      `json:"retry,omitempty"`
	Timeout         *TimeoutConfig    `json:"timeout,omitempty"`
	TLS             *TLSConfig        `json:"tls,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// Server represents an upstream server
type Server struct {
	ID       string            `json:"id"`
	Address  string            `json:"address"`
	Port     int               `json:"port"`
	Weight   int               `json:"weight"`
	Priority int               `json:"priority"`
	Status   ServerStatus      `json:"status"`
	Tags     map[string]string `json:"tags,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ServerStatus represents the status of an upstream server
type ServerStatus string

const (
	ServerStatusHealthy   ServerStatus = "healthy"
	ServerStatusUnhealthy ServerStatus = "unhealthy"
	ServerStatusDraining  ServerStatus = "draining"
	ServerStatusDisabled  ServerStatus = "disabled"
)

// HealthCheck represents health check configuration
type HealthCheck struct {
	Enabled            bool              `json:"enabled"`
	Interval           time.Duration     `json:"interval"`
	Timeout            time.Duration     `json:"timeout"`
	HealthyThreshold   int               `json:"healthy_threshold"`
	UnhealthyThreshold int               `json:"unhealthy_threshold"`
	Path               string            `json:"path,omitempty"`
	Method             string            `json:"method,omitempty"`
	Headers            map[string]string `json:"headers,omitempty"`
	ExpectedStatus     []int             `json:"expected_status,omitempty"`
	ExpectedBody       string            `json:"expected_body,omitempty"`
}

// CircuitBreaker represents circuit breaker configuration
type CircuitBreaker struct {
	Enabled          bool          `json:"enabled"`
	FailureThreshold int           `json:"failure_threshold"`
	RecoveryTimeout  time.Duration `json:"recovery_timeout"`
	SuccessThreshold int           `json:"success_threshold"`
	Timeout          time.Duration `json:"timeout"`
}

// RetryConfig represents retry configuration
type RetryConfig struct {
	Enabled     bool          `json:"enabled"`
	MaxRetries  int           `json:"max_retries"`
	RetryDelay  time.Duration `json:"retry_delay"`
	BackoffType string        `json:"backoff_type"`
	StatusCodes []int         `json:"status_codes,omitempty"`
}

// TimeoutConfig represents timeout configuration
type TimeoutConfig struct {
	Connect time.Duration `json:"connect"`
	Read    time.Duration `json:"read"`
	Write   time.Duration `json:"write"`
	Idle    time.Duration `json:"idle"`
}

// TLSConfig represents TLS configuration
type TLSConfig struct {
	Enabled            bool     `json:"enabled"`
	CertFile           string   `json:"cert_file,omitempty"`
	KeyFile            string   `json:"key_file,omitempty"`
	CAFile             string   `json:"ca_file,omitempty"`
	InsecureSkipVerify bool     `json:"insecure_skip_verify"`
	ServerName         string   `json:"server_name,omitempty"`
	CipherSuites       []string `json:"cipher_suites,omitempty"`
	MinVersion         string   `json:"min_version,omitempty"`
	MaxVersion         string   `json:"max_version,omitempty"`
}

// Matcher defines the interface for route matching
type Matcher interface {
	// Match checks if a request matches the criteria
	Match(req *http.Request, route *Route) (bool, error)
	
	// Name returns the matcher name
	Name() string
	
	// Priority returns the matcher priority
	Priority() int
	
	// Configure configures the matcher
	Configure(config map[string]interface{}) error
}

// PathMatcher defines the interface for path matching
type PathMatcher interface {
	Matcher
	
	// MatchPath matches a request path
	MatchPath(path string, pattern string) (bool, map[string]string, error)
	
	// ExtractParams extracts path parameters
	ExtractParams(path string, pattern string) (map[string]string, error)
}

// HostMatcher defines the interface for host matching
type HostMatcher interface {
	Matcher
	
	// MatchHost matches a request host
	MatchHost(host string, pattern string) (bool, error)
}

// HeaderMatcher defines the interface for header matching
type HeaderMatcher interface {
	Matcher
	
	// MatchHeaders matches request headers
	MatchHeaders(headers http.Header, patterns map[string]string) (bool, error)
}

// QueryMatcher defines the interface for query parameter matching
type QueryMatcher interface {
	Matcher
	
	// MatchQuery matches query parameters
	MatchQuery(query map[string][]string, patterns map[string]string) (bool, error)
}

// Manager defines the interface for router management
type Manager interface {
	// CreateRouter creates a new router instance
	CreateRouter(name string, config *Config) (Router, error)
	
	// GetRouter gets a router by name
	GetRouter(name string) (Router, error)
	
	// RemoveRouter removes a router
	RemoveRouter(name string) error
	
	// ListRouters lists all routers
	ListRouters() []string
	
	// RegisterMatcher registers a route matcher
	RegisterMatcher(name string, matcher Matcher) error
	
	// GetMatcher gets a matcher by name
	GetMatcher(name string) (Matcher, error)
	
	// ListMatchers lists all registered matchers
	ListMatchers() []string
	
	// LoadConfig loads routing configuration
	LoadConfig(config *Config) error
	
	// ReloadConfig reloads routing configuration
	ReloadConfig() error
}

// Config represents router configuration
type Config struct {
	Routes    []*Route               `json:"routes"`
	Upstreams []*Upstream            `json:"upstreams"`
	Matchers  []*MatcherConfig       `json:"matchers,omitempty"`
	Options   map[string]interface{} `json:"options,omitempty"`
}

// MatcherConfig represents matcher configuration
type MatcherConfig struct {
	Name     string                 `json:"name"`
	Type     string                 `json:"type"`
	Priority int                    `json:"priority,omitempty"`
	Config   map[string]interface{} `json:"config,omitempty"`
}

// Validator defines the interface for route validation
type Validator interface {
	// ValidateRoute validates a route
	ValidateRoute(route *Route) error
	
	// ValidateUpstream validates an upstream
	ValidateUpstream(upstream *Upstream) error
	
	// ValidateConfig validates the entire configuration
	ValidateConfig(config *Config) error
}

// Metrics defines the interface for router metrics
type Metrics interface {
	// RecordRequest records a request metric
	RecordRequest(route *Route, upstream *Upstream, server *Server, duration time.Duration, statusCode int)
	
	// GetRouteStats gets statistics for a route
	GetRouteStats(routeID string) (*RouteStats, error)
	
	// GetUpstreamStats gets statistics for an upstream
	GetUpstreamStats(upstreamID string) (*UpstreamStats, error)
	
	// GetOverallStats gets overall statistics
	GetOverallStats() (*OverallStats, error)
	
	// Reset resets all metrics
	Reset()
}

// RouteStats represents statistics for a route
type RouteStats struct {
	RouteID         string        `json:"route_id"`
	RequestCount    int64         `json:"request_count"`
	ErrorCount      int64         `json:"error_count"`
	AvgResponseTime time.Duration `json:"avg_response_time"`
	RequestRate     float64       `json:"request_rate"`
	ErrorRate       float64       `json:"error_rate"`
	StatusCodes     map[int]int64 `json:"status_codes"`
}

// UpstreamStats represents statistics for an upstream
type UpstreamStats struct {
	UpstreamID      string        `json:"upstream_id"`
	RequestCount    int64         `json:"request_count"`
	ErrorCount      int64         `json:"error_count"`
	AvgResponseTime time.Duration `json:"avg_response_time"`
	ActiveServers   int           `json:"active_servers"`
	TotalServers    int           `json:"total_servers"`
	ServerStats     map[string]*ServerStats `json:"server_stats"`
}

// ServerStats represents statistics for a server
type ServerStats struct {
	ServerID        string        `json:"server_id"`
	RequestCount    int64         `json:"request_count"`
	ErrorCount      int64         `json:"error_count"`
	AvgResponseTime time.Duration `json:"avg_response_time"`
	Status          ServerStatus  `json:"status"`
}

// OverallStats represents overall router statistics
type OverallStats struct {
	TotalRequests   int64         `json:"total_requests"`
	TotalErrors     int64         `json:"total_errors"`
	AvgResponseTime time.Duration `json:"avg_response_time"`
	RequestRate     float64       `json:"request_rate"`
	ErrorRate       float64       `json:"error_rate"`
	ActiveRoutes    int           `json:"active_routes"`
	TotalRoutes     int           `json:"total_routes"`
	ActiveUpstreams int           `json:"active_upstreams"`
	TotalUpstreams  int           `json:"total_upstreams"`
}
