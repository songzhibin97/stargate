package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/router"
	"github.com/songzhibin97/stargate/internal/store"
)

// PipelineInterface defines the interface for pipeline operations
type PipelineInterface interface {
	UpdateRoute(route *router.RouteRule) error
	DeleteRoute(routeID string) error
	UpdateUpstream(upstream *router.Upstream) error
	DeleteUpstream(upstreamID string) error
	RebuildMiddleware() error
	ReloadRoutes(routes []router.RouteRule) error
	ReloadUpstreams(upstreams []router.Upstream) error
}

// ConfigReloader handles dynamic configuration reloading for the data plane
type ConfigReloader struct {
	config     *config.Config
	store      store.Store
	pipeline   PipelineInterface
	mu         sync.RWMutex
	running    bool
	stopCh     chan struct{}
	wg         sync.WaitGroup
	lastUpdate time.Time
	version    string
}

// NewConfigReloader creates a new configuration reloader
func NewConfigReloader(cfg *config.Config, store store.Store, pipeline PipelineInterface) *ConfigReloader {
	return &ConfigReloader{
		config:   cfg,
		store:    store,
		pipeline: pipeline,
		stopCh:   make(chan struct{}),
	}
}

// Start starts the configuration reloader
func (cr *ConfigReloader) Start() error {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	if cr.running {
		return fmt.Errorf("config reloader is already running")
	}

	cr.running = true

	// Start watching for configuration changes
	if err := cr.startWatching(); err != nil {
		cr.running = false
		return fmt.Errorf("failed to start watching: %w", err)
	}

	// Start periodic full reload
	cr.wg.Add(1)
	go cr.periodicReload()

	log.Println("Configuration reloader started")
	return nil
}

// Stop stops the configuration reloader
func (cr *ConfigReloader) Stop() {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	if !cr.running {
		return
	}

	cr.running = false
	close(cr.stopCh)

	// Stop all watchers
	cr.store.Unwatch("routes/")
	cr.store.Unwatch("upstreams/")
	cr.store.Unwatch("plugins/")

	cr.wg.Wait()
	log.Println("Configuration reloader stopped")
}

// startWatching starts watching for configuration changes
func (cr *ConfigReloader) startWatching() error {
	// Watch routes
	if err := cr.store.Watch("routes/", cr.onRouteChange); err != nil {
		return fmt.Errorf("failed to watch routes: %w", err)
	}

	// Watch upstreams
	if err := cr.store.Watch("upstreams/", cr.onUpstreamChange); err != nil {
		return fmt.Errorf("failed to watch upstreams: %w", err)
	}

	// Watch plugins
	if err := cr.store.Watch("plugins/", cr.onPluginChange); err != nil {
		return fmt.Errorf("failed to watch plugins: %w", err)
	}

	return nil
}

// onRouteChange handles route configuration changes
func (cr *ConfigReloader) onRouteChange(key string, value []byte, eventType store.EventType) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	if !cr.running {
		return
	}

	log.Printf("Route configuration changed: key=%s, type=%v", key, eventType)

	switch eventType {
	case store.EventTypePut:
		var route router.RouteRule
		if err := json.Unmarshal(value, &route); err != nil {
			log.Printf("Failed to unmarshal route: %v", err)
			return
		}
		cr.updateRoute(&route)
	case store.EventTypeDelete:
		routeID := cr.extractIDFromKey(key, "routes/")
		cr.deleteRoute(routeID)
	}

	cr.lastUpdate = time.Now()
}

// onUpstreamChange handles upstream configuration changes
func (cr *ConfigReloader) onUpstreamChange(key string, value []byte, eventType store.EventType) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	if !cr.running {
		return
	}

	log.Printf("Upstream configuration changed: key=%s, type=%v", key, eventType)

	switch eventType {
	case store.EventTypePut:
		var upstream router.Upstream
		if err := json.Unmarshal(value, &upstream); err != nil {
			log.Printf("Failed to unmarshal upstream: %v", err)
			return
		}
		cr.updateUpstream(&upstream)
	case store.EventTypeDelete:
		upstreamID := cr.extractIDFromKey(key, "upstreams/")
		cr.deleteUpstream(upstreamID)
	}

	cr.lastUpdate = time.Now()
}

// onPluginChange handles plugin configuration changes
func (cr *ConfigReloader) onPluginChange(key string, value []byte, eventType store.EventType) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	if !cr.running {
		return
	}

	log.Printf("Plugin configuration changed: key=%s, type=%v", key, eventType)

	switch eventType {
	case store.EventTypePut:
		// Plugin configuration changed - trigger middleware chain rebuild
		cr.rebuildMiddlewareChain()
	case store.EventTypeDelete:
		// Plugin deleted - trigger middleware chain rebuild
		cr.rebuildMiddlewareChain()
	}

	cr.lastUpdate = time.Now()
}

// updateRoute updates a single route in the pipeline
func (cr *ConfigReloader) updateRoute(route *router.RouteRule) {
	if cr.pipeline == nil {
		return
	}

	// Update the route in the router
	if err := cr.pipeline.UpdateRoute(route); err != nil {
		log.Printf("Failed to update route %s: %v", route.ID, err)
		return
	}

	log.Printf("Route %s updated successfully", route.ID)
}

// deleteRoute removes a route from the pipeline
func (cr *ConfigReloader) deleteRoute(routeID string) {
	if cr.pipeline == nil {
		return
	}

	// Remove the route from the router
	if err := cr.pipeline.DeleteRoute(routeID); err != nil {
		log.Printf("Failed to delete route %s: %v", routeID, err)
		return
	}

	log.Printf("Route %s deleted successfully", routeID)
}

// updateUpstream updates a single upstream in the pipeline
func (cr *ConfigReloader) updateUpstream(upstream *router.Upstream) {
	if cr.pipeline == nil {
		return
	}

	// Update the upstream in the load balancer
	if err := cr.pipeline.UpdateUpstream(upstream); err != nil {
		log.Printf("Failed to update upstream %s: %v", upstream.ID, err)
		return
	}

	log.Printf("Upstream %s updated successfully", upstream.ID)
}

// deleteUpstream removes an upstream from the pipeline
func (cr *ConfigReloader) deleteUpstream(upstreamID string) {
	if cr.pipeline == nil {
		return
	}

	// Remove the upstream from the load balancer
	if err := cr.pipeline.DeleteUpstream(upstreamID); err != nil {
		log.Printf("Failed to delete upstream %s: %v", upstreamID, err)
		return
	}

	log.Printf("Upstream %s deleted successfully", upstreamID)
}

// rebuildMiddlewareChain rebuilds the entire middleware chain
func (cr *ConfigReloader) rebuildMiddlewareChain() {
	if cr.pipeline == nil {
		return
	}

	// Reload all plugin configurations and rebuild middleware chain
	if err := cr.pipeline.RebuildMiddleware(); err != nil {
		log.Printf("Failed to rebuild middleware chain: %v", err)
		return
	}

	log.Println("Middleware chain rebuilt successfully")
}

// periodicReload performs periodic full configuration reload
func (cr *ConfigReloader) periodicReload() {
	defer cr.wg.Done()

	ticker := time.NewTicker(30 * time.Second) // Full reload every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-cr.stopCh:
			return
		case <-ticker.C:
			cr.performFullReload()
		}
	}
}

// performFullReload performs a full configuration reload
func (cr *ConfigReloader) performFullReload() {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	if !cr.running {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Println("Performing full configuration reload...")

	// Load all routes
	if err := cr.reloadRoutes(ctx); err != nil {
		log.Printf("Failed to reload routes: %v", err)
	}

	// Load all upstreams
	if err := cr.reloadUpstreams(ctx); err != nil {
		log.Printf("Failed to reload upstreams: %v", err)
	}

	// Load all plugins
	if err := cr.reloadPlugins(ctx); err != nil {
		log.Printf("Failed to reload plugins: %v", err)
	}

	cr.lastUpdate = time.Now()
	log.Println("Full configuration reload completed")
}

// reloadRoutes reloads all route configurations
func (cr *ConfigReloader) reloadRoutes(ctx context.Context) error {
	routesData, err := cr.store.List(ctx, "routes/")
	if err != nil {
		return fmt.Errorf("failed to list routes: %w", err)
	}

	var routes []router.RouteRule
	for _, data := range routesData {
		var route router.RouteRule
		if err := json.Unmarshal(data, &route); err != nil {
			log.Printf("Failed to unmarshal route: %v", err)
			continue
		}
		routes = append(routes, route)
	}

	// Update all routes in the pipeline
	if err := cr.pipeline.ReloadRoutes(routes); err != nil {
		return fmt.Errorf("failed to reload routes in pipeline: %w", err)
	}

	log.Printf("Reloaded %d routes", len(routes))
	return nil
}

// reloadUpstreams reloads all upstream configurations
func (cr *ConfigReloader) reloadUpstreams(ctx context.Context) error {
	upstreamsData, err := cr.store.List(ctx, "upstreams/")
	if err != nil {
		return fmt.Errorf("failed to list upstreams: %w", err)
	}

	var upstreams []router.Upstream
	for _, data := range upstreamsData {
		var upstream router.Upstream
		if err := json.Unmarshal(data, &upstream); err != nil {
			log.Printf("Failed to unmarshal upstream: %v", err)
			continue
		}
		upstreams = append(upstreams, upstream)
	}

	// Update all upstreams in the pipeline
	if err := cr.pipeline.ReloadUpstreams(upstreams); err != nil {
		return fmt.Errorf("failed to reload upstreams in pipeline: %w", err)
	}

	log.Printf("Reloaded %d upstreams", len(upstreams))
	return nil
}

// reloadPlugins reloads all plugin configurations
func (cr *ConfigReloader) reloadPlugins(ctx context.Context) error {
	_, err := cr.store.List(ctx, "plugins/")
	if err != nil {
		return fmt.Errorf("failed to list plugins: %w", err)
	}

	// Trigger middleware chain rebuild
	if err := cr.pipeline.RebuildMiddleware(); err != nil {
		return fmt.Errorf("failed to rebuild middleware: %w", err)
	}

	log.Printf("Reloaded plugins and rebuilt middleware chain")
	return nil
}

// extractIDFromKey extracts the ID from a store key
func (cr *ConfigReloader) extractIDFromKey(key, prefix string) string {
	if len(key) > len(prefix) {
		return key[len(prefix):]
	}
	return ""
}

// GetStatus returns the status of the configuration reloader
func (cr *ConfigReloader) GetStatus() map[string]interface{} {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	return map[string]interface{}{
		"running":     cr.running,
		"last_update": cr.lastUpdate.Unix(),
		"version":     cr.version,
	}
}
