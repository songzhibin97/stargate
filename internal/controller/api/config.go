package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/router"
	"github.com/songzhibin97/stargate/internal/store"
)

// ConfigHandler handles configuration management API requests
type ConfigHandler struct {
	config *config.Config
	store  store.Store
}

// NewConfigHandler creates a new config handler
func NewConfigHandler(cfg *config.Config, store store.Store) *ConfigHandler {
	return &ConfigHandler{
		config: cfg,
		store:  store,
	}
}

// ConfigSnapshot represents a complete configuration snapshot
type ConfigSnapshot struct {
	Routes    []router.RouteRule `json:"routes"`
	Upstreams []router.Upstream  `json:"upstreams"`
	Plugins   []Plugin           `json:"plugins"`
	Timestamp int64              `json:"timestamp"`
	Version   string             `json:"version"`
}

// GetConfig handles GET /config
func (ch *ConfigHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := context.Background()
	
	// Get all routes
	var routes []router.RouteRule
	if routesData, err := ch.store.List(ctx, "routes/"); err == nil {
		for _, data := range routesData {
			var route router.RouteRule
			if err := json.Unmarshal(data, &route); err == nil {
				routes = append(routes, route)
			}
		}
	}

	// Get all upstreams
	var upstreams []router.Upstream
	if upstreamsData, err := ch.store.List(ctx, "upstreams/"); err == nil {
		for _, data := range upstreamsData {
			var upstream router.Upstream
			if err := json.Unmarshal(data, &upstream); err == nil {
				upstreams = append(upstreams, upstream)
			}
		}
	}

	// Get all plugins
	var plugins []Plugin
	if pluginsData, err := ch.store.List(ctx, "plugins/"); err == nil {
		for _, data := range pluginsData {
			var plugin Plugin
			if err := json.Unmarshal(data, &plugin); err == nil {
				plugins = append(plugins, plugin)
			}
		}
	}

	snapshot := ConfigSnapshot{
		Routes:    routes,
		Upstreams: upstreams,
		Plugins:   plugins,
		Timestamp: time.Now().Unix(),
		Version:   "1.0.0",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(snapshot)
}

// UpdateConfig handles PUT /config
func (ch *ConfigHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var snapshot ConfigSnapshot
	if err := json.NewDecoder(r.Body).Decode(&snapshot); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON format", err)
		return
	}

	ctx := context.Background()

	// Validate all routes
	for _, route := range snapshot.Routes {
		if err := route.Validate(); err != nil {
			writeErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("Route validation failed for %s", route.ID), err)
			return
		}
	}

	// Validate all upstreams
	for _, upstream := range snapshot.Upstreams {
		if err := upstream.Validate(); err != nil {
			writeErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("Upstream validation failed for %s", upstream.ID), err)
			return
		}
	}

	// Validate all plugins
	for _, plugin := range snapshot.Plugins {
		if err := plugin.Validate(); err != nil {
			writeErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("Plugin validation failed for %s", plugin.ID), err)
			return
		}
	}

	// Check route-upstream references
	upstreamIDs := make(map[string]bool)
	for _, upstream := range snapshot.Upstreams {
		upstreamIDs[upstream.ID] = true
	}

	for _, route := range snapshot.Routes {
		if !upstreamIDs[route.UpstreamID] {
			writeErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("Route %s references non-existent upstream %s", route.ID, route.UpstreamID), nil)
			return
		}
	}

	// Begin transaction-like operation
	// First, clear existing configuration
	if err := ch.clearExistingConfig(ctx); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to clear existing configuration", err)
		return
	}

	// Store new configuration
	if err := ch.storeNewConfig(ctx, &snapshot); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to store new configuration", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":   "Configuration updated successfully",
		"timestamp": time.Now().Unix(),
	})
}

// ValidateConfig handles POST /config/validate
func (ch *ConfigHandler) ValidateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var snapshot ConfigSnapshot
	if err := json.NewDecoder(r.Body).Decode(&snapshot); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON format", err)
		return
	}

	var validationErrors []string

	// Validate all routes
	for _, route := range snapshot.Routes {
		if err := route.Validate(); err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("Route %s: %s", route.ID, err.Error()))
		}
	}

	// Validate all upstreams
	for _, upstream := range snapshot.Upstreams {
		if err := upstream.Validate(); err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("Upstream %s: %s", upstream.ID, err.Error()))
		}
	}

	// Validate all plugins
	for _, plugin := range snapshot.Plugins {
		if err := plugin.Validate(); err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("Plugin %s: %s", plugin.ID, err.Error()))
		}
	}

	// Check route-upstream references
	upstreamIDs := make(map[string]bool)
	for _, upstream := range snapshot.Upstreams {
		upstreamIDs[upstream.ID] = true
	}

	for _, route := range snapshot.Routes {
		if !upstreamIDs[route.UpstreamID] {
			validationErrors = append(validationErrors, fmt.Sprintf("Route %s references non-existent upstream %s", route.ID, route.UpstreamID))
		}
	}

	response := map[string]interface{}{
		"valid": len(validationErrors) == 0,
	}

	if len(validationErrors) > 0 {
		response["errors"] = validationErrors
		w.WriteHeader(http.StatusBadRequest)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// clearExistingConfig removes all existing configuration
func (ch *ConfigHandler) clearExistingConfig(ctx context.Context) error {
	// Clear routes
	if routesData, err := ch.store.List(ctx, "routes/"); err == nil {
		for key := range routesData {
			if err := ch.store.Delete(ctx, key); err != nil {
				return fmt.Errorf("failed to delete route %s: %w", key, err)
			}
		}
	}

	// Clear upstreams
	if upstreamsData, err := ch.store.List(ctx, "upstreams/"); err == nil {
		for key := range upstreamsData {
			if err := ch.store.Delete(ctx, key); err != nil {
				return fmt.Errorf("failed to delete upstream %s: %w", key, err)
			}
		}
	}

	// Clear plugins
	if pluginsData, err := ch.store.List(ctx, "plugins/"); err == nil {
		for key := range pluginsData {
			if err := ch.store.Delete(ctx, key); err != nil {
				return fmt.Errorf("failed to delete plugin %s: %w", key, err)
			}
		}
	}

	return nil
}

// storeNewConfig stores the new configuration
func (ch *ConfigHandler) storeNewConfig(ctx context.Context, snapshot *ConfigSnapshot) error {
	// Store routes
	for _, route := range snapshot.Routes {
		route.SetTimestamps()
		data, err := json.Marshal(route)
		if err != nil {
			return fmt.Errorf("failed to marshal route %s: %w", route.ID, err)
		}
		key := fmt.Sprintf("routes/%s", route.ID)
		if err := ch.store.Put(ctx, key, data); err != nil {
			return fmt.Errorf("failed to store route %s: %w", route.ID, err)
		}
	}

	// Store upstreams
	for _, upstream := range snapshot.Upstreams {
		upstream.SetTimestamps()
		data, err := json.Marshal(upstream)
		if err != nil {
			return fmt.Errorf("failed to marshal upstream %s: %w", upstream.ID, err)
		}
		key := fmt.Sprintf("upstreams/%s", upstream.ID)
		if err := ch.store.Put(ctx, key, data); err != nil {
			return fmt.Errorf("failed to store upstream %s: %w", upstream.ID, err)
		}
	}

	// Store plugins
	for _, plugin := range snapshot.Plugins {
		plugin.SetTimestamps()
		data, err := json.Marshal(plugin)
		if err != nil {
			return fmt.Errorf("failed to marshal plugin %s: %w", plugin.ID, err)
		}
		key := fmt.Sprintf("plugins/%s", plugin.ID)
		if err := ch.store.Put(ctx, key, data); err != nil {
			return fmt.Errorf("failed to store plugin %s: %w", plugin.ID, err)
		}
	}

	return nil
}
