package router

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/songzhibin97/stargate/internal/config"
)

// Engine represents the routing engine
type Engine struct {
	config *config.Config
	mu     sync.RWMutex
	routes []*Route

	// Compiled route patterns for performance
	compiledRoutes []*compiledRoute

	// Enhanced router for PathRule support
	enhancedRouter *EnhancedRouter
}

// Route represents a routing rule
type Route struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Hosts      []string          `json:"hosts"`
	Paths      []string          `json:"paths"`
	Methods    []string          `json:"methods"`
	UpstreamID string            `json:"upstream_id"`
	Priority   int               `json:"priority"`
	Metadata   map[string]string `json:"metadata"`
	CreatedAt  int64             `json:"created_at"`
	UpdatedAt  int64             `json:"updated_at"`
}

// compiledRoute represents a compiled route with regex patterns
type compiledRoute struct {
	route       *Route
	hostRegexes []*regexp.Regexp
	pathRegexes []*regexp.Regexp
	methods     map[string]bool
}

// MatchResult represents the result of route matching
type MatchResult struct {
	Route    *Route
	Params   map[string]string
	Metadata map[string]string
}

// NewEngine creates a new routing engine
func NewEngine(cfg *config.Config) *Engine {
	return &Engine{
		config:         cfg,
		routes:         make([]*Route, 0),
		compiledRoutes: make([]*compiledRoute, 0),
		enhancedRouter: NewEnhancedRouter(),
	}
}

// Match finds the best matching route for the request
func (e *Engine) Match(r *http.Request) (*MatchResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	host := r.Host
	path := r.URL.Path
	method := r.Method

	// Remove port from host if present
	if colonIndex := strings.Index(host, ":"); colonIndex != -1 {
		host = host[:colonIndex]
	}

	// Try to match routes in priority order
	for _, compiled := range e.compiledRoutes {
		if e.matchRoute(compiled, host, path, method) {
			return &MatchResult{
				Route:    compiled.route,
				Params:   make(map[string]string), // TODO: Extract path parameters
				Metadata: compiled.route.Metadata,
			}, nil
		}
	}

	return nil, fmt.Errorf("no matching route found")
}

// MatchEnhanced 使用增强路由器匹配请求
func (e *Engine) MatchEnhanced(r *http.Request) (*EnhancedMatchResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := e.enhancedRouter.Match(r)
	if !result.Matched {
		return nil, fmt.Errorf("no matching route found")
	}

	return result, nil
}

// AddRouteRule 添加RouteRule到增强路由器
func (e *Engine) AddRouteRule(rule *RouteRule) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.enhancedRouter.AddRoute(rule)
}

// AddRouteRules 批量添加RouteRule
func (e *Engine) AddRouteRules(rules []RouteRule) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.enhancedRouter.AddRoutes(rules)
}

// RemoveRouteRule 从增强路由器移除RouteRule
func (e *Engine) RemoveRouteRule(routeID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.enhancedRouter.RemoveRoute(routeID)
}

// LoadFromConfigManager 从配置管理器加载路由规则
func (e *Engine) LoadFromConfigManager(cm *ConfigManager) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	config := cm.GetConfig()

	// 清空现有的增强路由
	e.enhancedRouter.Clear()

	// 添加新的路由规则
	for _, route := range config.Routes {
		if err := e.enhancedRouter.AddRoute(&route); err != nil {
			return fmt.Errorf("failed to add route %s: %w", route.ID, err)
		}
	}

	return nil
}

// UpdateRoute updates an existing route or adds a new one
func (e *Engine) UpdateRoute(route *RouteRule) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Remove existing route if it exists
	e.enhancedRouter.RemoveRoute(route.ID)

	// Add the updated route
	return e.enhancedRouter.AddRoute(route)
}

// DeleteRoute removes a route by ID
func (e *Engine) DeleteRoute(routeID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.enhancedRouter.RemoveRoute(routeID) {
		return fmt.Errorf("route %s not found", routeID)
	}

	return nil
}

// AddRoute adds a new route
func (e *Engine) AddRoute(route *RouteRule) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.enhancedRouter.AddRoute(route)
}

// ClearRoutes removes all routes
func (e *Engine) ClearRoutes() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.enhancedRouter.Clear()
	return nil
}

// GetRoute returns a route by ID
func (e *Engine) GetRoute(routeID string) (*RouteRule, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Search through enhanced routes
	for _, route := range e.enhancedRouter.GetRoutes() {
		if route.ID == routeID {
			return route.RouteRule, nil
		}
	}

	return nil, fmt.Errorf("route with ID %s not found", routeID)
}

// ListRoutes returns all routes
func (e *Engine) ListRoutes() []*RouteRule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Convert enhanced routes to RouteRule slice
	enhancedRoutes := e.enhancedRouter.GetRoutes()
	routes := make([]*RouteRule, len(enhancedRoutes))
	for i, route := range enhancedRoutes {
		routes[i] = route.RouteRule
	}

	return routes
}

// GetRouteCount returns the number of routes
func (e *Engine) GetRouteCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.enhancedRouter.Size()
}

// ReloadRoutes reloads all routes from a slice
func (e *Engine) ReloadRoutes(routes []RouteRule) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Clear existing routes
	e.enhancedRouter.Clear()

	// Add all new routes
	return e.enhancedRouter.AddRoutes(routes)
}

// AddLegacyRoute adds a new legacy route to the engine
func (e *Engine) AddLegacyRoute(route *Route) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Validate route
	if err := e.validateRoute(route); err != nil {
		return fmt.Errorf("invalid route: %w", err)
	}

	// Check for duplicate route ID
	for _, existing := range e.routes {
		if existing.ID == route.ID {
			return fmt.Errorf("route with ID %s already exists", route.ID)
		}
	}

	// Compile route patterns
	compiled, err := e.compileRoute(route)
	if err != nil {
		return fmt.Errorf("failed to compile route: %w", err)
	}

	// Add route
	e.routes = append(e.routes, route)
	e.compiledRoutes = append(e.compiledRoutes, compiled)

	// Sort routes by priority (higher priority first)
	e.sortRoutes()

	return nil
}

// UpdateLegacyRoute updates an existing legacy route
func (e *Engine) UpdateLegacyRoute(route *Route) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Find existing route
	index := -1
	for i, existing := range e.routes {
		if existing.ID == route.ID {
			index = i
			break
		}
	}

	if index == -1 {
		return fmt.Errorf("route with ID %s not found", route.ID)
	}

	// Validate route
	if err := e.validateRoute(route); err != nil {
		return fmt.Errorf("invalid route: %w", err)
	}

	// Compile route patterns
	compiled, err := e.compileRoute(route)
	if err != nil {
		return fmt.Errorf("failed to compile route: %w", err)
	}

	// Update route
	e.routes[index] = route
	e.compiledRoutes[index] = compiled

	// Sort routes by priority
	e.sortRoutes()

	return nil
}

// RemoveRoute removes a route from the engine
func (e *Engine) RemoveRoute(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Find route index
	index := -1
	for i, route := range e.routes {
		if route.ID == id {
			index = i
			break
		}
	}

	if index == -1 {
		return fmt.Errorf("route with ID %s not found", id)
	}

	// Remove route
	e.routes = append(e.routes[:index], e.routes[index+1:]...)
	e.compiledRoutes = append(e.compiledRoutes[:index], e.compiledRoutes[index+1:]...)

	return nil
}

// ListLegacyRoutes returns all legacy routes
func (e *Engine) ListLegacyRoutes() []*Route {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Create a copy to avoid race conditions
	routes := make([]*Route, len(e.routes))
	copy(routes, e.routes)
	return routes
}

// GetLegacyRoute returns a legacy route by ID
func (e *Engine) GetLegacyRoute(id string) (*Route, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, route := range e.routes {
		if route.ID == id {
			return route, nil
		}
	}

	return nil, fmt.Errorf("route with ID %s not found", id)
}

// matchRoute checks if a compiled route matches the request
func (e *Engine) matchRoute(compiled *compiledRoute, host, path, method string) bool {
	// Check method
	if len(compiled.methods) > 0 && !compiled.methods[method] {
		return false
	}

	// Check host
	if len(compiled.hostRegexes) > 0 {
		matched := false
		for _, regex := range compiled.hostRegexes {
			if regex.MatchString(host) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check path
	if len(compiled.pathRegexes) > 0 {
		matched := false
		for _, regex := range compiled.pathRegexes {
			if regex.MatchString(path) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

// compileRoute compiles route patterns into regex
func (e *Engine) compileRoute(route *Route) (*compiledRoute, error) {
	compiled := &compiledRoute{
		route:       route,
		hostRegexes: make([]*regexp.Regexp, 0),
		pathRegexes: make([]*regexp.Regexp, 0),
		methods:     make(map[string]bool),
	}

	// Compile host patterns
	for _, host := range route.Hosts {
		pattern := e.convertToRegex(host)
		regex, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid host pattern %s: %w", host, err)
		}
		compiled.hostRegexes = append(compiled.hostRegexes, regex)
	}

	// Compile path patterns
	for _, path := range route.Paths {
		pattern := e.convertToRegex(path)
		regex, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid path pattern %s: %w", path, err)
		}
		compiled.pathRegexes = append(compiled.pathRegexes, regex)
	}

	// Compile methods
	for _, method := range route.Methods {
		compiled.methods[strings.ToUpper(method)] = true
	}

	return compiled, nil
}

// convertToRegex converts a pattern with wildcards to regex
func (e *Engine) convertToRegex(pattern string) string {
	// Escape special regex characters except * and ?
	escaped := regexp.QuoteMeta(pattern)
	
	// Replace escaped wildcards with regex equivalents
	escaped = strings.ReplaceAll(escaped, "\\*", ".*")
	escaped = strings.ReplaceAll(escaped, "\\?", ".")
	
	// Anchor the pattern
	return "^" + escaped + "$"
}

// validateRoute validates a route configuration
func (e *Engine) validateRoute(route *Route) error {
	if route.ID == "" {
		return fmt.Errorf("route ID cannot be empty")
	}

	if route.Name == "" {
		return fmt.Errorf("route name cannot be empty")
	}

	if len(route.Hosts) == 0 && len(route.Paths) == 0 {
		return fmt.Errorf("route must have at least one host or path pattern")
	}

	if route.UpstreamID == "" {
		return fmt.Errorf("route must have an upstream ID")
	}

	return nil
}

// sortRoutes sorts routes by priority (higher priority first)
func (e *Engine) sortRoutes() {
	sort.Slice(e.routes, func(i, j int) bool {
		return e.routes[i].Priority > e.routes[j].Priority
	})

	sort.Slice(e.compiledRoutes, func(i, j int) bool {
		return e.compiledRoutes[i].route.Priority > e.compiledRoutes[j].route.Priority
	})
}

// Health returns the health status of the router
func (e *Engine) Health() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return map[string]interface{}{
		"status":      "healthy",
		"route_count": len(e.routes),
	}
}

// Metrics returns router metrics
func (e *Engine) Metrics() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return map[string]interface{}{
		"route_count": len(e.routes),
	}
}
