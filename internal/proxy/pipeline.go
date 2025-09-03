package proxy

import (
	"context"
	"fmt"
	"hash/fnv"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/songzhibin97/stargate/internal/auth"
	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/governance/circuitbreaker"
	"github.com/songzhibin97/stargate/internal/governance/trafficmirror"
	"github.com/songzhibin97/stargate/internal/health"
	"github.com/songzhibin97/stargate/internal/loadbalancer"
	"github.com/songzhibin97/stargate/internal/middleware"
	"github.com/songzhibin97/stargate/internal/ratelimit"
	"github.com/songzhibin97/stargate/internal/router"
	"github.com/songzhibin97/stargate/internal/types"
	"github.com/songzhibin97/stargate/pkg/metrics"
)

// Pipeline represents the request processing pipeline
type Pipeline struct {
	config    *config.Config
	startTime time.Time
	mu        sync.RWMutex
	logger    *log.Logger

	// Middleware chain
	middlewares []Middleware
	middlewareManager *middleware.Manager

	// Core components
	router                   Router
	loadBalancer             types.LoadBalancer
	loadBalancerManager      *loadbalancer.Manager
	reverseProxy             *ReverseProxy
	websocketProxy           *WebSocketProxy
	passiveHealthChecker     *health.PassiveHealthChecker
	authMiddleware           *auth.Middleware
	ipaclMiddleware          *middleware.IPACLMiddleware
	corsMiddleware           *middleware.CORSMiddleware
	headerTransformMiddleware *middleware.HeaderTransformMiddleware
	mockResponseMiddleware   *middleware.MockResponseMiddleware
	grpcWebMiddleware        *middleware.GRPCWebMiddleware
	rateLimitMiddleware      *ratelimit.Middleware
	circuitBreakerMiddleware *circuitbreaker.Middleware
	trafficMirrorMiddleware  *trafficmirror.Middleware
	accessLogMiddleware      *middleware.AccessLogMiddleware
	metricsMiddleware        *middleware.MetricsMiddleware
	tracingMiddleware        *middleware.TracingMiddleware
	aggregatorMiddleware     *middleware.AggregatorMiddleware
	serverlessMiddleware     *middleware.ServerlessMiddleware
	wasmMiddleware           *middleware.WASMMiddleware

	// Metrics
	requestCount  int64
	responseCount int64
	errorCount    int64
}

// Middleware represents a middleware function
type Middleware func(http.Handler) http.Handler

// Router interface for route matching
type Router interface {
	Match(r *http.Request) (*Route, error)
	AddRoute(route *Route) error
	UpdateRoute(route *router.RouteRule) error
	DeleteRoute(id string) error
	RemoveRoute(id string) error
	ClearRoutes() error
	ListRoutes() []*Route
}



// Route represents a routing rule
type Route struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Hosts      []string          `json:"hosts"`
	Paths      []string          `json:"paths"`
	Methods    []string          `json:"methods"`
	UpstreamID string            `json:"upstream_id"`
	Metadata   map[string]string `json:"metadata"`
	CreatedAt  int64             `json:"created_at"`
	UpdatedAt  int64             `json:"updated_at"`
}



// MockRouter is a temporary mock implementation of Router interface
type MockRouter struct{}

// Match implements Router interface
func (m *MockRouter) Match(r *http.Request) (*Route, error) {
	// Return a default route for testing
	return &Route{
		ID:         "default",
		Name:       "Default Route",
		Hosts:      []string{"*"},
		Paths:      []string{"/*"},
		Methods:    []string{"GET", "POST", "PUT", "DELETE"},
		UpstreamID: "default-upstream",
	}, nil
}

// AddRoute implements Router interface
func (m *MockRouter) AddRoute(route *Route) error {
	return nil
}

// RemoveRoute implements Router interface
func (m *MockRouter) RemoveRoute(id string) error {
	return nil
}

// UpdateRoute implements Router interface
func (m *MockRouter) UpdateRoute(route *router.RouteRule) error {
	return nil
}

// DeleteRoute implements Router interface
func (m *MockRouter) DeleteRoute(id string) error {
	return nil
}

// ClearRoutes implements Router interface
func (m *MockRouter) ClearRoutes() error {
	return nil
}

// ListRoutes implements Router interface
func (m *MockRouter) ListRoutes() []*Route {
	return []*Route{}
}

// NewPipeline creates a new request processing pipeline
func NewPipeline(cfg *config.Config, logger *log.Logger) (*Pipeline, error) {
	if logger == nil {
		logger = log.New(log.Writer(), "[proxy.pipeline] ", log.LstdFlags)
	}

	p := &Pipeline{
		config:    cfg,
		startTime: time.Now(),
		logger:    logger,
	}

	// Initialize components
	if err := p.initializeComponents(); err != nil {
		return nil, fmt.Errorf("failed to initialize components: %w", err)
	}

	// Build middleware chain
	if err := p.buildMiddlewareChain(); err != nil {
		return nil, fmt.Errorf("failed to build middleware chain: %w", err)
	}

	return p, nil
}

// UpdateRoute updates a single route in the pipeline
func (p *Pipeline) UpdateRoute(route *router.RouteRule) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.router == nil {
		return fmt.Errorf("router not initialized")
	}

	// Update the route in the router
	return p.router.UpdateRoute(route)
}

// DeleteRoute removes a route from the pipeline
func (p *Pipeline) DeleteRoute(routeID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.router == nil {
		return fmt.Errorf("router not initialized")
	}

	// Remove the route from the router
	return p.router.DeleteRoute(routeID)
}

// UpdateUpstream updates a single upstream in the pipeline
func (p *Pipeline) UpdateUpstream(upstream *router.Upstream) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.loadBalancerManager == nil {
		return fmt.Errorf("load balancer manager not initialized")
	}

	// Update the upstream in the load balancer manager
	return p.loadBalancerManager.UpdateUpstream(upstream)
}

// DeleteUpstream removes an upstream from the pipeline
func (p *Pipeline) DeleteUpstream(upstreamID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.loadBalancerManager == nil {
		return fmt.Errorf("load balancer manager not initialized")
	}

	// Remove the upstream from the load balancer manager
	return p.loadBalancerManager.DeleteUpstream(upstreamID)
}

// RebuildMiddleware rebuilds the entire middleware chain
func (p *Pipeline) RebuildMiddleware() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.middlewareManager == nil {
		return fmt.Errorf("middleware manager not initialized")
	}

	log.Println("Rebuilding middleware chain...")

	// Rebuild the middleware chain using the manager
	return p.middlewareManager.RebuildChain()
}

// ReloadRoutes reloads all routes in the pipeline
func (p *Pipeline) ReloadRoutes(routes []router.RouteRule) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.router == nil {
		return fmt.Errorf("router not initialized")
	}

	// Clear existing routes and reload all
	if err := p.router.ClearRoutes(); err != nil {
		return fmt.Errorf("failed to clear existing routes: %w", err)
	}

	// Add all routes
	for _, route := range routes {
		// Convert router.RouteRule to Route
		proxyRoute := &Route{
			ID:         route.ID,
			Name:       route.Name,
			Hosts:      route.Rules.Hosts,
			Paths:      convertPathRulesToStrings(route.Rules.Paths),
			Methods:    route.Rules.Methods,
			UpstreamID: route.UpstreamID,
			Metadata:   route.Metadata,
			CreatedAt:  route.CreatedAt,
			UpdatedAt:  route.UpdatedAt,
		}

		if err := p.router.AddRoute(proxyRoute); err != nil {
			log.Printf("Failed to add route %s: %v", route.ID, err)
			continue
		}
	}

	log.Printf("Reloaded %d routes in pipeline", len(routes))
	return nil
}

// convertPathRulesToStrings converts router.PathRule slice to string slice
func convertPathRulesToStrings(pathRules []router.PathRule) []string {
	paths := make([]string, len(pathRules))
	for i, rule := range pathRules {
		paths[i] = rule.Value
	}
	return paths
}

// ReloadUpstreams reloads all upstreams in the pipeline
func (p *Pipeline) ReloadUpstreams(upstreams []router.Upstream) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.loadBalancerManager == nil {
		return fmt.Errorf("load balancer manager not initialized")
	}

	// Reload all upstreams using the manager
	return p.loadBalancerManager.ReloadUpstreams(upstreams)
}

// ServeHTTP implements http.Handler interface
func (p *Pipeline) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	p.requestCount++
	p.mu.Unlock()

	// Handle metrics endpoint
	if p.config.Metrics.Enabled && r.URL.Path == p.config.Metrics.Path {
		if p.metricsMiddleware != nil {
			// Use the provider's HTTP handler for metrics endpoint
			if provider := p.getMetricsProvider(); provider != nil {
				provider.Handler().ServeHTTP(w, r)
				return
			}
		}
		// Fallback to default Prometheus handler for backward compatibility
		promhttp.Handler().ServeHTTP(w, r)
		return
	}

	// Log protocol information for debugging
	p.logProtocolInfo(r)

	// Check if this is a WebSocket upgrade request
	if p.websocketProxy.IsWebSocketUpgrade(r) {
		// Handle WebSocket upgrade
		if err := p.websocketProxy.HandleWebSocketUpgrade(w, r); err != nil {
			p.handleError(w, r, http.StatusBadRequest, fmt.Sprintf("WebSocket upgrade failed: %v", err))
			return
		}
		// WebSocket upgrade handled, connection hijacked
		return
	}

	// Create handler chain for regular HTTP requests
	handler := p.createHandler()

	// Execute middleware chain
	for i := len(p.middlewares) - 1; i >= 0; i-- {
		handler = p.middlewares[i](handler)
	}

	// Serve request
	handler.ServeHTTP(w, r)

	p.mu.Lock()
	p.responseCount++
	p.mu.Unlock()
}

// logProtocolInfo logs detailed protocol information for debugging
func (p *Pipeline) logProtocolInfo(r *http.Request) {
	protocol := "HTTP/1.1"
	if r.ProtoMajor == 2 {
		protocol = "HTTP/2"
	}

	log.Printf("Proxy request received: method=%s path=%s proto=%s protocol=%s tls=%v remote=%s",
		r.Method,
		r.URL.Path,
		r.Proto,
		protocol,
		r.TLS != nil,
		r.RemoteAddr,
	)

	// Log additional HTTP/2 specific information
	if r.ProtoMajor == 2 {
		log.Printf("HTTP/2 connection details: stream_multiplexing active=%v", true)
		if r.TLS != nil {
			log.Printf("HTTP/2 over TLS: alpn_negotiation successful=%v", true)
		}
	}
}

// Start starts the pipeline
func (p *Pipeline) Start() error {
	// Start load balancer
	if starter, ok := p.loadBalancer.(interface{ Start() error }); ok {
		if err := starter.Start(); err != nil {
			return fmt.Errorf("failed to start load balancer: %w", err)
		}
	}

	// Start router
	if starter, ok := p.router.(interface{ Start() error }); ok {
		if err := starter.Start(); err != nil {
			return fmt.Errorf("failed to start router: %w", err)
		}
	}

	return nil
}

// Stop stops the pipeline
func (p *Pipeline) Stop() error {
	// Stop WebSocket proxy
	if p.websocketProxy != nil {
		if err := p.websocketProxy.Close(); err != nil {
			log.Printf("Failed to close WebSocket proxy: %v", err)
		}
	}

	// Stop load balancer
	if stopper, ok := p.loadBalancer.(interface{ Stop() error }); ok {
		if err := stopper.Stop(); err != nil {
			return fmt.Errorf("failed to stop load balancer: %w", err)
		}
	}

	// Stop router
	if stopper, ok := p.router.(interface{ Stop() error }); ok {
		if err := stopper.Stop(); err != nil {
			return fmt.Errorf("failed to stop router: %w", err)
		}
	}

	// Stop passive health checker
	if p.passiveHealthChecker != nil {
		if err := p.passiveHealthChecker.Stop(); err != nil {
			log.Printf("Failed to stop passive health checker: %v", err)
		}
	}

	// Stop rate limit middleware
	if p.rateLimitMiddleware != nil {
		p.rateLimitMiddleware.Stop()
	}

	return nil
}

// Health returns pipeline health status
func (p *Pipeline) Health() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	health := map[string]interface{}{
		"status":         "healthy",
		"uptime":         time.Since(p.startTime).Seconds(),
		"request_count":  p.requestCount,
		"response_count": p.responseCount,
		"error_count":    p.errorCount,
	}

	// Add load balancer health
	if p.loadBalancer != nil {
		health["load_balancer"] = p.loadBalancer.Health()
	}

	return health
}

// Metrics returns pipeline metrics
func (p *Pipeline) Metrics() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return map[string]interface{}{
		"uptime":         time.Since(p.startTime).Seconds(),
		"request_count":  p.requestCount,
		"response_count": p.responseCount,
		"error_count":    p.errorCount,
	}
}

// Reload reloads the pipeline configuration
func (p *Pipeline) Reload(cfg *config.Config) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config = cfg

	// Rebuild middleware chain
	return p.buildMiddlewareChain()
}

// initializeComponents initializes pipeline components
func (p *Pipeline) initializeComponents() error {
	// Initialize router
	p.router = &MockRouter{}

	// Initialize load balancer based on configuration
	p.loadBalancer = p.createLoadBalancer()

	// Initialize reverse proxy
	var err error
	p.reverseProxy, err = NewReverseProxy(p.config)
	if err != nil {
		return fmt.Errorf("failed to create reverse proxy: %w", err)
	}

	// Initialize WebSocket proxy
	p.websocketProxy = NewWebSocketProxy(p.config)

	// Initialize passive health checker
	passiveConfig := p.convertToPassiveHealthConfig()
	p.passiveHealthChecker = health.NewPassiveHealthChecker(passiveConfig, p.onHealthStatusChange)
	if err := p.passiveHealthChecker.Start(); err != nil {
		return fmt.Errorf("failed to start passive health checker: %w", err)
	}

	// Initialize authentication middleware
	if p.config.Auth.Enabled {
		p.authMiddleware = auth.NewMiddleware(&p.config.Auth)
	}

	// Initialize IP ACL middleware
	if p.config.IPACL.Enabled {
		ipaclMiddleware, err := middleware.NewIPACLMiddleware(&p.config.IPACL)
		if err != nil {
			return fmt.Errorf("failed to create IP ACL middleware: %w", err)
		}
		p.ipaclMiddleware = ipaclMiddleware
	}

	// Initialize CORS middleware
	if p.config.CORS.Enabled {
		p.corsMiddleware = middleware.NewCORSMiddleware(&p.config.CORS)
	}

	// Initialize header transform middleware
	if p.config.HeaderTransform.Enabled {
		p.headerTransformMiddleware = middleware.NewHeaderTransformMiddleware(&p.config.HeaderTransform)
	}

	// Initialize mock response middleware
	if p.config.MockResponse.Enabled {
		p.mockResponseMiddleware, err = middleware.NewMockResponseMiddleware(&p.config.MockResponse)
		if err != nil {
			return fmt.Errorf("failed to create mock response middleware: %w", err)
		}
	}

	// Initialize gRPC-Web middleware
	if p.config.GRPCWeb.Enabled {
		p.grpcWebMiddleware, err = middleware.NewGRPCWebMiddleware(&p.config.GRPCWeb)
		if err != nil {
			return fmt.Errorf("failed to create gRPC-Web middleware: %w", err)
		}
	}

	// Initialize rate limit middleware
	if p.config.RateLimit.Enabled {
		rateLimitConfig := p.convertToRateLimitConfig()
		p.rateLimitMiddleware, err = ratelimit.NewMiddleware(rateLimitConfig)
		if err != nil {
			return fmt.Errorf("failed to create rate limit middleware: %w", err)
		}
	}

	// Initialize circuit breaker middleware
	if p.config.CircuitBreaker.Enabled {
		p.circuitBreakerMiddleware, err = circuitbreaker.NewMiddleware(&p.config.CircuitBreaker)
		if err != nil {
			return fmt.Errorf("failed to create circuit breaker middleware: %w", err)
		}
	}

	// Initialize traffic mirror middleware
	if p.config.TrafficMirror.Enabled {
		p.trafficMirrorMiddleware, err = trafficmirror.NewMiddleware(&p.config.TrafficMirror)
		if err != nil {
			return fmt.Errorf("failed to create traffic mirror middleware: %w", err)
		}
	}

	// Initialize access log middleware
	if p.config.Logging.AccessLog.Enabled {
		p.accessLogMiddleware, err = middleware.NewAccessLogMiddleware(&p.config.Logging.AccessLog)
		if err != nil {
			return fmt.Errorf("failed to create access log middleware: %w", err)
		}
	}

	// Initialize metrics middleware
	if p.config.Metrics.Enabled {
		// Check if using new unified config or legacy Prometheus config
		if p.config.Metrics.Provider != "" || len(p.config.Metrics.EnabledMetrics) > 0 {
			// Use new unified metrics configuration
			metricsConfig := &middleware.MetricsConfig{
				Enabled:         p.config.Metrics.Enabled,
				Provider:        p.config.Metrics.Provider,
				Namespace:       p.config.Metrics.Namespace,
				Subsystem:       p.config.Metrics.Subsystem,
				ConstLabels:     p.config.Metrics.ConstLabels,
				EnabledMetrics:  p.config.Metrics.EnabledMetrics,
				CustomLabels:    p.config.Metrics.CustomLabels,
				SampleRate:      p.config.Metrics.SampleRate,
				LabelExtractors: p.config.Metrics.LabelExtractors,
				SensitiveLabels: p.config.Metrics.SensitiveLabels,
				MaxLabelLength:  p.config.Metrics.MaxLabelLength,
				AsyncUpdates:    p.config.Metrics.AsyncUpdates,
				BufferSize:      p.config.Metrics.BufferSize,
			}

			// Set defaults if not specified
			if metricsConfig.Provider == "" {
				metricsConfig.Provider = "prometheus"
			}
			if metricsConfig.Namespace == "" {
				metricsConfig.Namespace = "stargate"
			}
			if metricsConfig.Subsystem == "" {
				metricsConfig.Subsystem = "node"
			}
			if metricsConfig.SampleRate == 0 {
				metricsConfig.SampleRate = 1.0
			}
			if metricsConfig.MaxLabelLength == 0 {
				metricsConfig.MaxLabelLength = 256
			}
			if metricsConfig.BufferSize == 0 {
				metricsConfig.BufferSize = 1000
			}
			if metricsConfig.EnabledMetrics == nil {
				metricsConfig.EnabledMetrics = map[string]bool{
					"requests_total":     true,
					"request_duration":   true,
					"request_size":       true,
					"response_size":      true,
					"active_connections": true,
					"errors_total":       true,
				}
			}

			p.metricsMiddleware, err = middleware.NewMetricsMiddlewareFromConfig(metricsConfig)
			if err != nil {
				return fmt.Errorf("failed to create metrics middleware: %w", err)
			}
		} else if p.config.Metrics.Prometheus.Enabled {
			// Use legacy Prometheus configuration for backward compatibility
			p.metricsMiddleware, err = middleware.NewMetricsMiddlewareFromPrometheusConfig(&p.config.Metrics.Prometheus)
			if err != nil {
				return fmt.Errorf("failed to create metrics middleware from Prometheus config: %w", err)
			}
		}
	}

	// Initialize tracing middleware
	if p.config.Tracing.Enabled {
		p.tracingMiddleware, err = middleware.NewTracingMiddleware(&p.config.Tracing)
		if err != nil {
			return fmt.Errorf("failed to create tracing middleware: %w", err)
		}
	}

	// Initialize aggregator middleware
	if p.config.Aggregator.Enabled {
		p.aggregatorMiddleware = middleware.NewAggregatorMiddleware(&p.config.Aggregator)
	}

	// Initialize serverless middleware
	if p.config.Serverless.Enabled {
		p.serverlessMiddleware = middleware.NewServerlessMiddleware(&p.config.Serverless)
	}

	// Initialize WASM middleware
	if p.config.WASM.Enabled {
		var err error
		p.wasmMiddleware, err = middleware.NewWASMMiddleware(&p.config.WASM)
		if err != nil {
			return fmt.Errorf("failed to create WASM middleware: %w", err)
		}
	}

	return nil
}

// convertToPassiveHealthConfig converts config to passive health config
func (p *Pipeline) convertToPassiveHealthConfig() *health.PassiveHealthConfig {
	if p.config.Upstreams.Defaults.HealthCheck.Passive.Enabled {
		return &health.PassiveHealthConfig{
			Enabled:              p.config.Upstreams.Defaults.HealthCheck.Passive.Enabled,
			ConsecutiveFailures:  p.config.Upstreams.Defaults.HealthCheck.Passive.ConsecutiveFailures,
			IsolationDuration:    p.config.Upstreams.Defaults.HealthCheck.Passive.IsolationDuration,
			RecoveryInterval:     p.config.Upstreams.Defaults.HealthCheck.Passive.RecoveryInterval,
			ConsecutiveSuccesses: p.config.Upstreams.Defaults.HealthCheck.Passive.ConsecutiveSuccesses,
			FailureStatusCodes:   p.config.Upstreams.Defaults.HealthCheck.Passive.FailureStatusCodes,
			TimeoutAsFailure:     p.config.Upstreams.Defaults.HealthCheck.Passive.TimeoutAsFailure,
		}
	}
	return nil
}

// convertToRateLimitConfig converts config to rate limit config
func (p *Pipeline) convertToRateLimitConfig() *ratelimit.Config {
	// Convert strategy string to enum
	var strategy ratelimit.RateLimitStrategy
	switch p.config.RateLimit.Strategy {
	case "sliding_window":
		strategy = ratelimit.StrategySlidingWindow
	case "token_bucket":
		strategy = ratelimit.StrategyTokenBucket
	case "leaky_bucket":
		strategy = ratelimit.StrategyLeakyBucket
	default:
		strategy = ratelimit.StrategyFixedWindow
	}

	// Convert identifier strategy string to enum
	var identifierStrategy ratelimit.IdentifierStrategy
	switch p.config.RateLimit.IdentifierStrategy {
	case "user":
		identifierStrategy = ratelimit.IdentifierUser
	case "api_key":
		identifierStrategy = ratelimit.IdentifierAPIKey
	case "combined":
		identifierStrategy = ratelimit.IdentifierCombined
	default:
		identifierStrategy = ratelimit.IdentifierIP
	}

	return &ratelimit.Config{
		Strategy:               strategy,
		IdentifierStrategy:     identifierStrategy,
		WindowSize:             p.config.RateLimit.WindowSize,
		MaxRequests:            p.config.RateLimit.DefaultRate,
		Rate:                   float64(p.config.RateLimit.DefaultRate),
		BurstSize:              p.config.RateLimit.Burst,
		CleanupInterval:        p.config.RateLimit.CleanupInterval,
		Enabled:                p.config.RateLimit.Enabled,
		SkipSuccessfulRequests: p.config.RateLimit.SkipSuccessful,
		SkipFailedRequests:     p.config.RateLimit.SkipFailed,
		CustomHeaders:          p.config.RateLimit.CustomHeaders,
	}
}

// onHealthStatusChange handles health status changes from passive health checker
func (p *Pipeline) onHealthStatusChange(upstreamID, targetKey string, healthy bool) {
	log.Printf("Health status changed for %s in upstream %s: healthy=%v", targetKey, upstreamID, healthy)

	// Extract host and port from targetKey (format: upstreamID:host:port)
	parts := strings.Split(targetKey, ":")
	if len(parts) >= 3 {
		host := parts[1]
		port := 0
		if p, err := strconv.Atoi(parts[2]); err == nil {
			port = p
		}

		// Update load balancer
		if err := p.UpdateTargetHealth(upstreamID, host, port, healthy); err != nil {
			log.Printf("Failed to update target health in load balancer: %v", err)
		}
	}
}

// buildMiddlewareChain builds the middleware chain
func (p *Pipeline) buildMiddlewareChain() error {
	p.middlewares = []Middleware{}

	// Add tracing middleware (first to capture all requests in traces)
	if p.config.Tracing.Enabled && p.tracingMiddleware != nil {
		p.middlewares = append(p.middlewares, p.tracingMiddleware.Handler())
	}

	// Add access log middleware (second to log all requests)
	if p.config.Logging.AccessLog.Enabled && p.accessLogMiddleware != nil {
		p.middlewares = append(p.middlewares, p.accessLogMiddleware.Handler())
	}

	// Add metrics middleware (early to capture all metrics)
	if p.config.Metrics.Enabled && p.metricsMiddleware != nil {
		p.middlewares = append(p.middlewares, p.metricsMiddleware.Handler())
	}

	// Add CORS middleware (first in chain to handle preflight requests early)
	if p.config.CORS.Enabled && p.corsMiddleware != nil {
		p.middlewares = append(p.middlewares, p.corsMiddleware.Handler())
	}

	// Add header transform middleware (early in chain to transform headers before other processing)
	if p.config.HeaderTransform.Enabled && p.headerTransformMiddleware != nil {
		p.middlewares = append(p.middlewares, p.headerTransformMiddleware.Handler())
	}

	// Add mock response middleware (early in chain to return mock responses before backend processing)
	if p.config.MockResponse.Enabled && p.mockResponseMiddleware != nil {
		p.middlewares = append(p.middlewares, p.mockResponseMiddleware.Handler())
	}

	// Add gRPC-Web middleware (early in chain to handle gRPC-Web protocol conversion)
	if p.config.GRPCWeb.Enabled && p.grpcWebMiddleware != nil {
		p.middlewares = append(p.middlewares, p.grpcWebMiddleware.Handler())
	}

	// Add IP ACL middleware (after CORS for early IP-based rejection)
	if p.config.IPACL.Enabled && p.ipaclMiddleware != nil {
		p.middlewares = append(p.middlewares, p.ipaclMiddleware.Handler())
	}

	// Add rate limiting middleware (after IP ACL, before auth to limit unauthenticated requests)
	if p.config.RateLimit.Enabled && p.rateLimitMiddleware != nil {
		p.middlewares = append(p.middlewares, p.rateLimitMiddleware.Handler())
	}

	// Add auth middleware (after rate limiting)
	if p.config.Auth.Enabled && p.authMiddleware != nil {
		p.middlewares = append(p.middlewares, p.authMiddleware.Handler())
	}

	// Add aggregator middleware (after auth, before circuit breaker to handle aggregate requests)
	if p.config.Aggregator.Enabled && p.aggregatorMiddleware != nil {
		p.middlewares = append(p.middlewares, p.aggregatorMiddleware.Handler())
	}

	// Add serverless middleware (after aggregator, before circuit breaker for request/response processing)
	if p.config.Serverless.Enabled && p.serverlessMiddleware != nil {
		p.middlewares = append(p.middlewares, p.serverlessMiddleware.Handler())
	}

	// Add WASM middleware (after serverless, before circuit breaker for plugin processing)
	if p.config.WASM.Enabled && p.wasmMiddleware != nil {
		p.middlewares = append(p.middlewares, p.wasmMiddleware.Handler())
	}

	// Add circuit breaker middleware (after auth, before actual request processing)
	if p.config.CircuitBreaker.Enabled && p.circuitBreakerMiddleware != nil {
		p.middlewares = append(p.middlewares, p.circuitBreakerMiddleware.Handler())
	}

	// Add traffic mirror middleware (last in chain, after all processing)
	if p.config.TrafficMirror.Enabled && p.trafficMirrorMiddleware != nil {
		p.middlewares = append(p.middlewares, p.trafficMirrorMiddleware.Handler())
	}

	return nil
}

// getMetricsProvider returns the metrics provider from the middleware
func (p *Pipeline) getMetricsProvider() metrics.Provider {
	if p.metricsMiddleware == nil {
		return nil
	}

	return p.metricsMiddleware.GetProvider()
}

// createLoadBalancer 根据配置创建负载均衡器
func (p *Pipeline) createLoadBalancer() types.LoadBalancer {
	// 默认使用轮询策略
	algorithm := "round_robin"

	// 如果配置中指定了算法，使用配置的算法
	if p.config != nil && p.config.LoadBalancer.DefaultAlgorithm != "" {
		algorithm = p.config.LoadBalancer.DefaultAlgorithm
	}

	switch algorithm {
	case "canary":
		return loadbalancer.NewCanaryBalancer(p.config)
	case "weighted_round_robin":
		return loadbalancer.NewWeightedRoundRobinBalancer(p.config)
	case "ip_hash":
		return loadbalancer.NewIPHashBalancer(p.config)
	case "round_robin":
		fallthrough
	default:
		return loadbalancer.NewRoundRobinBalancer(p.config)
	}
}

// selectTarget 根据负载均衡器类型选择目标实例
func (p *Pipeline) selectTarget(upstream *types.Upstream, r *http.Request) (*types.Target, error) {
	// 对于IP Hash负载均衡器，需要特殊处理
	if lb, ok := p.loadBalancer.(*loadbalancer.IPHashBalancer); ok {
		return p.selectTargetWithIPHash(lb, upstream, r)
	}

	// 对于其他负载均衡器，使用标准Select方法
	return p.loadBalancer.Select(upstream)
}

// selectTargetWithIPHash 使用IP Hash负载均衡器选择目标
func (p *Pipeline) selectTargetWithIPHash(lb *loadbalancer.IPHashBalancer, upstream *types.Upstream, r *http.Request) (*types.Target, error) {
	// 提取客户端IP
	clientIP := loadbalancer.ExtractClientIP(r)

	// 获取健康的目标实例
	healthyTargets := make([]*types.Target, 0)
	for _, target := range upstream.Targets {
		if target.Healthy {
			healthyTargets = append(healthyTargets, target)
		}
	}

	if len(healthyTargets) == 0 {
		return nil, fmt.Errorf("no healthy targets available for upstream %s", upstream.ID)
	}

	// 使用IP哈希选择目标
	return p.selectByIPHash(clientIP, healthyTargets), nil
}

// selectByIPHash 使用IP哈希算法选择目标
func (p *Pipeline) selectByIPHash(clientIP string, targets []*types.Target) *types.Target {
	if len(targets) == 0 {
		return nil
	}

	// 使用与IPHashBalancer相同的哈希算法
	hash := fnv.New32a()
	hash.Write([]byte(clientIP))
	hashValue := hash.Sum32()

	// 简单取模选择目标
	index := hashValue % uint32(len(targets))
	return targets[index]
}

// getUpstream retrieves upstream by ID from load balancer
func (p *Pipeline) getUpstream(upstreamID string) *types.Upstream {
	// 尝试 CanaryBalancer
	if lb, ok := p.loadBalancer.(*loadbalancer.CanaryBalancer); ok {
		upstream, err := lb.GetUpstream(upstreamID)
		if err != nil {
			return nil
		}
		return upstream
	}

	// 尝试 RoundRobinBalancer
	if lb, ok := p.loadBalancer.(*loadbalancer.RoundRobinBalancer); ok {
		upstream, err := lb.GetUpstream(upstreamID)
		if err != nil {
			return nil
		}
		return upstream
	}

	// 尝试 WeightedRoundRobinBalancer
	if lb, ok := p.loadBalancer.(*loadbalancer.WeightedRoundRobinBalancer); ok {
		upstream, err := lb.GetUpstream(upstreamID)
		if err != nil {
			return nil
		}
		return upstream
	}

	// 尝试 IPHashBalancer
	if lb, ok := p.loadBalancer.(*loadbalancer.IPHashBalancer); ok {
		upstream, err := lb.GetUpstream(upstreamID)
		if err != nil {
			return nil
		}
		return upstream
	}

	return nil
}

// AddUpstream adds an upstream to the load balancer
func (p *Pipeline) AddUpstream(upstream *types.Upstream) error {
	return p.loadBalancer.UpdateUpstream(upstream)
}

// RemoveUpstream removes an upstream from the load balancer
func (p *Pipeline) RemoveUpstream(upstreamID string) error {
	return p.loadBalancer.RemoveUpstream(upstreamID)
}

// UpdateTargetHealth updates the health status of a target
func (p *Pipeline) UpdateTargetHealth(upstreamID, targetHost string, targetPort int, healthy bool) error {
	// 尝试 CanaryBalancer
	if lb, ok := p.loadBalancer.(*loadbalancer.CanaryBalancer); ok {
		return lb.UpdateTargetHealth(upstreamID, targetHost, targetPort, healthy)
	}

	// 尝试 RoundRobinBalancer
	if lb, ok := p.loadBalancer.(*loadbalancer.RoundRobinBalancer); ok {
		return lb.UpdateTargetHealth(upstreamID, targetHost, targetPort, healthy)
	}

	// 尝试 WeightedRoundRobinBalancer
	if lb, ok := p.loadBalancer.(*loadbalancer.WeightedRoundRobinBalancer); ok {
		return lb.UpdateTargetHealth(upstreamID, targetHost, targetPort, healthy)
	}

	// 尝试 IPHashBalancer
	if lb, ok := p.loadBalancer.(*loadbalancer.IPHashBalancer); ok {
		return lb.UpdateTargetHealth(upstreamID, targetHost, targetPort, healthy)
	}

	return fmt.Errorf("load balancer does not support health updates")
}

// createHandler creates the core request handler
func (p *Pipeline) createHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		// Route matching
		route, err := p.router.Match(r)
		if err != nil {
			p.handleError(w, r, http.StatusNotFound, "route not found")
			return
		}

		// Add route ID to request context for circuit breaker
		ctx := context.WithValue(r.Context(), "route_id", route.ID)
		r = r.WithContext(ctx)

		// Get upstream for the matched route
		upstream := p.getUpstream(route.UpstreamID)
		if upstream == nil {
			p.handleError(w, r, http.StatusBadGateway, "upstream not found")
			return
		}

		// Load balancing - select target from upstream
		target, err := p.selectTarget(upstream, r)
		if err != nil {
			p.handleError(w, r, http.StatusServiceUnavailable, fmt.Sprintf("load balancer error: %v", err))
			return
		}

		// Set target in request context for reverse proxy
		r = SetTarget(r, target)

		// Wrap response writer to capture status code
		wrapper := NewResponseWrapper(w)

		// Reverse proxy
		p.reverseProxy.ServeHTTP(wrapper, r)

		// Record request result for passive health checking
		if p.passiveHealthChecker != nil {
			// Check if there was a proxy error
			var proxyError error
			var isTimeout bool
			if err, ok := r.Context().Value("proxy_error").(error); ok {
				proxyError = err
			}
			if timeout, ok := r.Context().Value("proxy_timeout").(bool); ok {
				isTimeout = timeout
			}

			result := &health.RequestResult{
				UpstreamID: upstream.ID,
				Target:     target,
				StatusCode: wrapper.StatusCode(),
				Error:      proxyError,
				Duration:   wrapper.Duration(),
				IsTimeout:  isTimeout,
				Timestamp:  startTime,
			}
			p.passiveHealthChecker.RecordRequest(result)
		}
	})
}

// handleError handles errors
func (p *Pipeline) handleError(w http.ResponseWriter, r *http.Request, status int, message string) {
	p.mu.Lock()
	p.errorCount++
	p.mu.Unlock()

	w.WriteHeader(status)
	w.Write([]byte(message))
}

// Middleware implementations (placeholders)
func (p *Pipeline) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		// Log request details
		_ = time.Since(start)
	})
}





// rateLimitMiddleware is now handled by the dedicated rate limit middleware
// The old placeholder function has been removed
