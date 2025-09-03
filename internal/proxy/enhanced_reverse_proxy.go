package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/router"
	"github.com/songzhibin97/stargate/internal/types"
)

// EnhancedReverseProxy represents an enhanced reverse proxy with full routing integration
type EnhancedReverseProxy struct {
	*ReverseProxy
	routerAdapter *RouterAdapter
	upstreams     map[string]*types.Upstream
}

// NewEnhancedReverseProxy creates a new enhanced reverse proxy
func NewEnhancedReverseProxy(cfg *config.Config, routerAdapter *RouterAdapter) (*EnhancedReverseProxy, error) {
	// Create base reverse proxy
	baseProxy, err := NewReverseProxy(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create base reverse proxy: %w", err)
	}

	return &EnhancedReverseProxy{
		ReverseProxy:  baseProxy,
		routerAdapter: routerAdapter,
		upstreams:     make(map[string]*types.Upstream),
	}, nil
}

// ServeHTTP implements http.Handler interface with full routing integration
func (erp *EnhancedReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Match route using the enhanced router
	result := erp.routerAdapter.GetEnhancedRouter().Match(r)
	if !result.Matched {
		erp.handleError(w, r, http.StatusNotFound, "No matching route found")
		return
	}

	// Get upstream for the matched route
	upstream, exists := erp.upstreams[result.Route.UpstreamID]
	if !exists {
		erp.handleError(w, r, http.StatusBadGateway, fmt.Sprintf("Upstream %s not found", result.Route.UpstreamID))
		return
	}

	// Select target from upstream (simple round-robin for now)
	target := erp.selectTarget(upstream)
	if target == nil {
		erp.handleError(w, r, http.StatusServiceUnavailable, "No healthy targets available")
		return
	}

	// Set target in request context
	r = SetTarget(r, target)

	// Add route metadata to request context
	ctx := context.WithValue(r.Context(), "route", result.Route)
	ctx = context.WithValue(ctx, "upstream", upstream)
	r = r.WithContext(ctx)

	// Forward request using base reverse proxy
	erp.ReverseProxy.ServeHTTP(w, r)
}

// AddUpstream adds an upstream service
func (erp *EnhancedReverseProxy) AddUpstream(upstream *types.Upstream) {
	erp.upstreams[upstream.ID] = upstream
}

// RemoveUpstream removes an upstream service
func (erp *EnhancedReverseProxy) RemoveUpstream(upstreamID string) {
	delete(erp.upstreams, upstreamID)
}

// GetUpstream returns an upstream service by ID
func (erp *EnhancedReverseProxy) GetUpstream(upstreamID string) (*types.Upstream, bool) {
	upstream, exists := erp.upstreams[upstreamID]
	return upstream, exists
}

// ListUpstreams returns all upstream services
func (erp *EnhancedReverseProxy) ListUpstreams() map[string]*types.Upstream {
	return erp.upstreams
}

// selectTarget selects a target from upstream using simple round-robin
func (erp *EnhancedReverseProxy) selectTarget(upstream *types.Upstream) *types.Target {
	healthyTargets := make([]*types.Target, 0)
	
	// Filter healthy targets
	for _, target := range upstream.Targets {
		if target.Healthy {
			healthyTargets = append(healthyTargets, target)
		}
	}

	if len(healthyTargets) == 0 {
		return nil
	}

	// Simple round-robin selection (in a real implementation, this would be more sophisticated)
	return healthyTargets[0]
}

// handleError handles proxy errors
func (erp *EnhancedReverseProxy) handleError(w http.ResponseWriter, r *http.Request, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(fmt.Sprintf(`{"error": "%s", "message": "%s", "path": "%s"}`, 
		http.StatusText(status), message, r.URL.Path)))
}

// LoadUpstreamsFromConfig loads upstream services from configuration
func (erp *EnhancedReverseProxy) LoadUpstreamsFromConfig(configPath string) error {
	cm := router.NewConfigManager()

	if err := cm.LoadFromFile(configPath); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	config := cm.GetConfig()

	// Clear existing upstreams
	erp.upstreams = make(map[string]*types.Upstream)

	// Add upstreams from config
	for _, upstream := range config.Upstreams {
		erp.AddUpstream(&types.Upstream{
			ID:        upstream.ID,
			Name:      upstream.Name,
			Algorithm: upstream.Algorithm,
			Targets:   erp.convertConfigTargets(upstream.Targets),
			Metadata:  upstream.Metadata,
		})
	}

	return nil
}

// convertConfigTargets converts config targets to proxy targets
func (erp *EnhancedReverseProxy) convertConfigTargets(configTargets []router.Target) []*types.Target {
	targets := make([]*types.Target, len(configTargets))
	for i, configTarget := range configTargets {
		// Parse URL to extract host and port
		host, port := erp.parseTargetURL(configTarget.URL)
		targets[i] = &types.Target{
			Host:    host,
			Port:    port,
			Weight:  configTarget.Weight,
			Healthy: true, // Default to healthy
		}
	}
	return targets
}

// parseTargetURL parses a target URL to extract host and port
func (erp *EnhancedReverseProxy) parseTargetURL(targetURL string) (string, int) {
	// Simple URL parsing - in production, use url.Parse
	if strings.HasPrefix(targetURL, "http://") {
		hostPort := strings.TrimPrefix(targetURL, "http://")
		parts := strings.Split(hostPort, ":")
		if len(parts) == 2 {
			port := 80
			fmt.Sscanf(parts[1], "%d", &port)
			return parts[0], port
		}
		return hostPort, 80
	} else if strings.HasPrefix(targetURL, "https://") {
		hostPort := strings.TrimPrefix(targetURL, "https://")
		parts := strings.Split(hostPort, ":")
		if len(parts) == 2 {
			port := 443
			fmt.Sscanf(parts[1], "%d", &port)
			return parts[0], port
		}
		return hostPort, 443
	}
	// Default case
	return targetURL, 80
}

// CreateDefaultUpstream creates a default upstream for testing
func (erp *EnhancedReverseProxy) CreateDefaultUpstream() {
	defaultUpstream := &types.Upstream{
		ID:        "default-upstream",
		Name:      "Default Upstream",
		Algorithm: "round_robin",
		Targets: []*types.Target{
			{
				Host:    "httpbin.org",
				Port:    80,
				Weight:  100,
				Healthy: true,
			},
		},
		Metadata: map[string]string{
			"description": "Default upstream for testing",
		},
	}

	erp.AddUpstream(defaultUpstream)
}

// Health returns the health status of the enhanced reverse proxy
func (erp *EnhancedReverseProxy) Health() map[string]interface{} {
	baseHealth := erp.ReverseProxy.Health()
	
	// Add upstream health information
	upstreamHealth := make(map[string]interface{})
	for id, upstream := range erp.upstreams {
		healthyCount := 0
		totalCount := len(upstream.Targets)
		
		for _, target := range upstream.Targets {
			if target.Healthy {
				healthyCount++
			}
		}
		
		upstreamHealth[id] = map[string]interface{}{
			"name":          upstream.Name,
			"algorithm":     upstream.Algorithm,
			"total_targets": totalCount,
			"healthy_targets": healthyCount,
			"status":        func() string {
				if healthyCount == 0 {
					return "unhealthy"
				} else if healthyCount == totalCount {
					return "healthy"
				}
				return "degraded"
			}(),
		}
	}
	
	baseHealth["upstreams"] = upstreamHealth
	return baseHealth
}

// GetRouterAdapter returns the router adapter
func (erp *EnhancedReverseProxy) GetRouterAdapter() *RouterAdapter {
	return erp.routerAdapter
}

// MatchRoute matches a route for the given request (for testing purposes)
func (erp *EnhancedReverseProxy) MatchRoute(r *http.Request) (*router.EnhancedMatchResult, error) {
	result := erp.routerAdapter.GetEnhancedRouter().Match(r)
	if !result.Matched {
		return nil, fmt.Errorf("no matching route found")
	}
	return result, nil
}

// SetTargetFromURL sets target from URL string (utility function)
func (erp *EnhancedReverseProxy) SetTargetFromURL(r *http.Request, targetURL string) (*http.Request, error) {
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid target URL: %w", err)
	}

	port := 80
	if parsedURL.Scheme == "https" {
		port = 443
	}
	if parsedURL.Port() != "" {
		// Port is already specified in the URL
		if parsedURL.Scheme == "https" && parsedURL.Port() == "443" {
			port = 443
		} else if parsedURL.Scheme == "http" && parsedURL.Port() == "80" {
			port = 80
		} else {
			// Custom port
			fmt.Sscanf(parsedURL.Port(), "%d", &port)
		}
	}

	target := &types.Target{
		Host:    strings.Split(parsedURL.Host, ":")[0],
		Port:    port,
		Weight:  100,
		Healthy: true,
	}

	return SetTarget(r, target), nil
}
