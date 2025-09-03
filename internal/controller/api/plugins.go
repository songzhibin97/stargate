package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/store"
)

// Plugin represents a plugin configuration
type Plugin struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Type        string                 `json:"type"` // "middleware", "wasm", "auth", etc.
	Enabled     bool                   `json:"enabled"`
	Config      map[string]interface{} `json:"config"`
	Routes      []string               `json:"routes,omitempty"`      // Route IDs this plugin applies to
	Upstreams   []string               `json:"upstreams,omitempty"`   // Upstream IDs this plugin applies to
	Priority    int                    `json:"priority"`              // Execution priority
	Metadata    map[string]string      `json:"metadata,omitempty"`
	CreatedAt   int64                  `json:"created_at"`
	UpdatedAt   int64                  `json:"updated_at"`
}

// PluginHandler handles plugin management API requests
type PluginHandler struct {
	config         *config.Config
	store          store.Store
	configNotifier ConfigNotifier
}

// NewPluginHandler creates a new plugin handler
func NewPluginHandler(cfg *config.Config, store store.Store, configNotifier ConfigNotifier) *PluginHandler {
	return &PluginHandler{
		config:         cfg,
		store:          store,
		configNotifier: configNotifier,
	}
}

// Validate validates plugin configuration
func (p *Plugin) Validate() error {
	if p.ID == "" {
		return fmt.Errorf("plugin ID is required")
	}
	if p.Name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if p.Type == "" {
		return fmt.Errorf("plugin type is required")
	}

	// Validate plugin type
	validTypes := map[string]bool{
		"auth":           true,
		"rate_limit":     true,
		"cors":           true,
		"circuit_breaker": true,
		"traffic_mirror": true,
		"header_transform": true,
		"mock_response":  true,
		"wasm":           true,
		"custom":         true,
	}
	if !validTypes[p.Type] {
		return fmt.Errorf("invalid plugin type: %s", p.Type)
	}

	return nil
}

// SetTimestamps sets creation and update timestamps
func (p *Plugin) SetTimestamps() {
	now := time.Now().Unix()
	if p.CreatedAt == 0 {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
}

// CreatePlugin handles POST /plugins
func (ph *PluginHandler) CreatePlugin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var plugin Plugin
	if err := json.NewDecoder(r.Body).Decode(&plugin); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON format", err)
		return
	}

	// Generate ID if not provided
	if plugin.ID == "" {
		plugin.ID = generatePluginID()
	}

	// Set timestamps
	plugin.SetTimestamps()

	// Validate plugin
	if err := plugin.Validate(); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Plugin validation failed", err)
		return
	}

	// Check if plugin ID already exists
	ctx := context.Background()
	key := fmt.Sprintf("plugins/%s", plugin.ID)
	if _, err := ph.store.Get(ctx, key); err == nil {
		writeErrorResponse(w, http.StatusConflict, "Plugin ID already exists", nil)
		return
	}

	// Serialize plugin
	data, err := json.Marshal(plugin)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to serialize plugin", err)
		return
	}

	// Store plugin
	if err := ph.store.Put(ctx, key, data); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to store plugin", err)
		return
	}

	// Return created plugin
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Plugin created successfully",
		"plugin":  plugin,
	})
}

// GetPlugin handles GET /plugins/{id}
func (ph *PluginHandler) GetPlugin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract plugin ID from URL
	pluginID := extractPluginID(r.URL.Path)
	if pluginID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "Plugin ID is required", nil)
		return
	}

	ctx := context.Background()
	key := fmt.Sprintf("plugins/%s", pluginID)
	
	data, err := ph.store.Get(ctx, key)
	if err != nil {
		writeErrorResponse(w, http.StatusNotFound, "Plugin not found", err)
		return
	}

	var plugin Plugin
	if err := json.Unmarshal(data, &plugin); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to deserialize plugin", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(plugin)
}

// UpdatePlugin handles PUT /plugins/{id}
func (ph *PluginHandler) UpdatePlugin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract plugin ID from URL
	pluginID := extractPluginID(r.URL.Path)
	if pluginID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "Plugin ID is required", nil)
		return
	}

	var plugin Plugin
	if err := json.NewDecoder(r.Body).Decode(&plugin); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON format", err)
		return
	}

	// Ensure ID matches URL
	plugin.ID = pluginID

	// Update timestamp
	plugin.UpdatedAt = time.Now().Unix()

	// Validate plugin
	if err := plugin.Validate(); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Plugin validation failed", err)
		return
	}

	ctx := context.Background()
	key := fmt.Sprintf("plugins/%s", pluginID)

	// Check if plugin exists
	if _, err := ph.store.Get(ctx, key); err != nil {
		writeErrorResponse(w, http.StatusNotFound, "Plugin not found", err)
		return
	}

	// Serialize plugin
	data, err := json.Marshal(plugin)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to serialize plugin", err)
		return
	}

	// Update plugin
	if err := ph.store.Put(ctx, key, data); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to update plugin", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Plugin updated successfully",
		"plugin":  plugin,
	})
}

// DeletePlugin handles DELETE /plugins/{id}
func (ph *PluginHandler) DeletePlugin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract plugin ID from URL
	pluginID := extractPluginID(r.URL.Path)
	if pluginID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "Plugin ID is required", nil)
		return
	}

	ctx := context.Background()
	key := fmt.Sprintf("plugins/%s", pluginID)

	// Check if plugin exists
	if _, err := ph.store.Get(ctx, key); err != nil {
		writeErrorResponse(w, http.StatusNotFound, "Plugin not found", err)
		return
	}

	// Delete plugin
	if err := ph.store.Delete(ctx, key); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to delete plugin", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Plugin deleted successfully",
	})
}

// ListPlugins handles GET /plugins
func (ph *PluginHandler) ListPlugins(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := context.Background()
	
	// Get all plugins
	pluginsData, err := ph.store.List(ctx, "plugins/")
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to list plugins", err)
		return
	}

	var plugins []Plugin
	for _, data := range pluginsData {
		var plugin Plugin
		if err := json.Unmarshal(data, &plugin); err != nil {
			// Log error but continue with other plugins
			continue
		}
		plugins = append(plugins, plugin)
	}

	// Parse query parameters for filtering and pagination
	query := r.URL.Query()
	
	// Filter by type
	pluginType := query.Get("type")
	if pluginType != "" {
		var filtered []Plugin
		for _, plugin := range plugins {
			if plugin.Type == pluginType {
				filtered = append(filtered, plugin)
			}
		}
		plugins = filtered
	}

	// Filter by enabled status
	enabledStr := query.Get("enabled")
	if enabledStr != "" {
		enabled := enabledStr == "true"
		var filtered []Plugin
		for _, plugin := range plugins {
			if plugin.Enabled == enabled {
				filtered = append(filtered, plugin)
			}
		}
		plugins = filtered
	}

	// Pagination
	limit := 50 // default limit
	if l := query.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	offset := 0
	if o := query.Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Apply pagination
	total := len(plugins)
	start := offset
	end := offset + limit
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	paginatedPlugins := plugins[start:end]

	response := map[string]interface{}{
		"plugins": paginatedPlugins,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Helper functions
func extractPluginID(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 3 && parts[len(parts)-2] == "plugins" {
		return parts[len(parts)-1]
	}
	return ""
}

func generatePluginID() string {
	return fmt.Sprintf("plugin-%d", time.Now().UnixNano())
}
