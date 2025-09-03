package proxy

import (
	"fmt"
	"net/http"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/router"
)

// RouterAdapter adapts the EnhancedRouter to work with the proxy pipeline
type RouterAdapter struct {
	enhancedRouter *router.EnhancedRouter
}

// NewRouterAdapter creates a new router adapter
func NewRouterAdapter() *RouterAdapter {
	return &RouterAdapter{
		enhancedRouter: router.NewEnhancedRouter(),
	}
}

// Match implements the Router interface for the proxy pipeline
func (ra *RouterAdapter) Match(r *http.Request) (*Route, error) {
	result := ra.enhancedRouter.Match(r)
	if !result.Matched {
		return nil, fmt.Errorf("no matching route found")
	}

	// Convert EnhancedRoute to proxy Route
	proxyRoute := &Route{
		ID:         result.Route.ID,
		Name:       result.Route.Name,
		Hosts:      result.Route.Rules.Hosts,
		Paths:      ra.convertPathRulesToStrings(result.Route.Rules.Paths),
		Methods:    result.Route.Rules.Methods,
		UpstreamID: result.Route.UpstreamID,
		Metadata:   result.Route.Metadata,
		CreatedAt:  result.Route.CreatedAt,
		UpdatedAt:  result.Route.UpdatedAt,
	}

	return proxyRoute, nil
}

// AddRoute implements the Router interface for the proxy pipeline
func (ra *RouterAdapter) AddRoute(route *Route) error {
	// Convert proxy Route to RouteRule
	routeRule := &router.RouteRule{
		ID:         route.ID,
		Name:       route.Name,
		Rules: router.Rule{
			Hosts:   route.Hosts,
			Paths:   ra.convertStringsToPaths(route.Paths),
			Methods: route.Methods,
		},
		UpstreamID: route.UpstreamID,
		Priority:   100, // Default priority
		Metadata:   route.Metadata,
		CreatedAt:  route.CreatedAt,
		UpdatedAt:  route.UpdatedAt,
	}

	return ra.enhancedRouter.AddRoute(routeRule)
}

// RemoveRoute implements the Router interface for the proxy pipeline
func (ra *RouterAdapter) RemoveRoute(id string) error {
	if !ra.enhancedRouter.RemoveRoute(id) {
		return fmt.Errorf("route with ID %s not found", id)
	}
	return nil
}

// ListRoutes implements the Router interface for the proxy pipeline
func (ra *RouterAdapter) ListRoutes() []*Route {
	enhancedRoutes := ra.enhancedRouter.GetRoutes()
	proxyRoutes := make([]*Route, len(enhancedRoutes))

	for i, enhancedRoute := range enhancedRoutes {
		proxyRoutes[i] = &Route{
			ID:         enhancedRoute.ID,
			Name:       enhancedRoute.Name,
			Hosts:      enhancedRoute.Rules.Hosts,
			Paths:      ra.convertPathRulesToStrings(enhancedRoute.Rules.Paths),
			Methods:    enhancedRoute.Rules.Methods,
			UpstreamID: enhancedRoute.UpstreamID,
			Metadata:   enhancedRoute.Metadata,
			CreatedAt:  enhancedRoute.CreatedAt,
			UpdatedAt:  enhancedRoute.UpdatedAt,
		}
	}

	return proxyRoutes
}

// UpdateRoute implements the Router interface for updating routes
func (ra *RouterAdapter) UpdateRoute(rule *router.RouteRule) error {
	// Remove existing route if it exists
	ra.enhancedRouter.RemoveRoute(rule.ID)
	// Add the updated route
	return ra.enhancedRouter.AddRoute(rule)
}

// DeleteRoute implements the Router interface for deleting routes
func (ra *RouterAdapter) DeleteRoute(id string) error {
	if !ra.enhancedRouter.RemoveRoute(id) {
		return fmt.Errorf("route with ID %s not found", id)
	}
	return nil
}

// ClearRoutes implements the Router interface for clearing all routes
func (ra *RouterAdapter) ClearRoutes() error {
	ra.enhancedRouter.Clear()
	return nil
}

// AddRouteRule adds a RouteRule directly to the enhanced router
func (ra *RouterAdapter) AddRouteRule(rule *router.RouteRule) error {
	return ra.enhancedRouter.AddRoute(rule)
}

// AddRouteRules adds multiple RouteRules to the enhanced router
func (ra *RouterAdapter) AddRouteRules(rules []router.RouteRule) error {
	return ra.enhancedRouter.AddRoutes(rules)
}

// LoadFromConfigManager loads routes from a config manager
func (ra *RouterAdapter) LoadFromConfigManager(cm *router.ConfigManager) error {
	config := cm.GetConfig()

	// Clear existing routes
	ra.enhancedRouter.Clear()

	// Add new routes
	for _, route := range config.Routes {
		if err := ra.enhancedRouter.AddRoute(&route); err != nil {
			return fmt.Errorf("failed to add route %s: %w", route.ID, err)
		}
	}

	return nil
}

// GetEnhancedRouter returns the underlying enhanced router
func (ra *RouterAdapter) GetEnhancedRouter() *router.EnhancedRouter {
	return ra.enhancedRouter
}

// convertPathRulesToStrings converts PathRule slice to string slice
func (ra *RouterAdapter) convertPathRulesToStrings(pathRules []router.PathRule) []string {
	paths := make([]string, len(pathRules))
	for i, pathRule := range pathRules {
		paths[i] = pathRule.Value
	}
	return paths
}

// convertStringsToPaths converts string slice to PathRule slice
func (ra *RouterAdapter) convertStringsToPaths(paths []string) []router.PathRule {
	pathRules := make([]router.PathRule, len(paths))
	for i, path := range paths {
		// Determine match type based on path pattern
		matchType := router.MatchTypePrefix
		if path[len(path)-1] != '*' && path[len(path)-1] != '/' {
			matchType = router.MatchTypeExact
		}
		
		pathRules[i] = router.PathRule{
			Type:  matchType,
			Value: path,
		}
	}
	return pathRules
}

// EnhancedPipeline represents an enhanced request processing pipeline with full router integration
type EnhancedPipeline struct {
	*Pipeline
	routerAdapter *RouterAdapter
}

// NewEnhancedPipeline creates a new enhanced pipeline with integrated router
func NewEnhancedPipeline(cfg *config.Config) (*EnhancedPipeline, error) {
	// Create base pipeline
	basePipeline, err := NewPipeline(cfg, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create base pipeline: %w", err)
	}

	// Create router adapter
	routerAdapter := NewRouterAdapter()

	// Replace the mock router with our adapter
	basePipeline.router = routerAdapter

	return &EnhancedPipeline{
		Pipeline:      basePipeline,
		routerAdapter: routerAdapter,
	}, nil
}

// GetRouterAdapter returns the router adapter
func (ep *EnhancedPipeline) GetRouterAdapter() *RouterAdapter {
	return ep.routerAdapter
}

// LoadRoutesFromConfig loads routes from configuration
func (ep *EnhancedPipeline) LoadRoutesFromConfig(configPath string) error {
	cm := router.NewConfigManager()

	if err := cm.LoadFromFile(configPath); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	return ep.routerAdapter.LoadFromConfigManager(cm)
}

// AddRoute adds a route to the pipeline
func (ep *EnhancedPipeline) AddRoute(rule *router.RouteRule) error {
	return ep.routerAdapter.AddRouteRule(rule)
}

// AddRoutes adds multiple routes to the pipeline
func (ep *EnhancedPipeline) AddRoutes(rules []router.RouteRule) error {
	return ep.routerAdapter.AddRouteRules(rules)
}

// RemoveRoute removes a route from the pipeline
func (ep *EnhancedPipeline) RemoveRoute(routeID string) error {
	return ep.routerAdapter.RemoveRoute(routeID)
}

// ListRoutes lists all routes in the pipeline
func (ep *EnhancedPipeline) ListRoutes() []*Route {
	return ep.routerAdapter.ListRoutes()
}

// GetEnhancedRoutes returns all enhanced routes
func (ep *EnhancedPipeline) GetEnhancedRoutes() []*router.EnhancedRoute {
	return ep.routerAdapter.GetEnhancedRouter().GetRoutes()
}
