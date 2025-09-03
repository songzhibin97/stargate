package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/router"
	"github.com/songzhibin97/stargate/internal/store"
)

// RouteHandler handles route management API requests
type RouteHandler struct {
	config         *config.Config
	store          store.Store
	configNotifier ConfigNotifier
}

// ConfigNotifier interface for configuration change notifications
type ConfigNotifier interface {
	PublishConfigChange(changeType string, key string, value, oldValue []byte, source string) error
}

// NewRouteHandler creates a new route handler
func NewRouteHandler(cfg *config.Config, store store.Store, configNotifier ConfigNotifier) *RouteHandler {
	return &RouteHandler{
		config:         cfg,
		store:          store,
		configNotifier: configNotifier,
	}
}

// CreateRoute handles POST /routes
func (rh *RouteHandler) CreateRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var route router.RouteRule
	if err := json.NewDecoder(r.Body).Decode(&route); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON format", err)
		return
	}

	// Generate ID if not provided
	if route.ID == "" {
		route.ID = generateRouteID()
	}

	// Set timestamps
	route.SetTimestamps()

	// Validate route
	if err := route.Validate(); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Route validation failed", err)
		return
	}

	// Check if route ID already exists
	ctx := context.Background()
	key := fmt.Sprintf("routes/%s", route.ID)
	if _, err := rh.store.Get(ctx, key); err == nil {
		writeErrorResponse(w, http.StatusConflict, "Route ID already exists", nil)
		return
	}

	// Serialize route
	data, err := json.Marshal(route)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to serialize route", err)
		return
	}

	// Store route
	if err := rh.store.Put(ctx, key, data); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to store route", err)
		return
	}

	// Notify configuration change
	if rh.configNotifier != nil {
		if err := rh.configNotifier.PublishConfigChange("create", key, data, nil, "admin_api"); err != nil {
			// Log error but don't fail the request
			log.Printf("Failed to publish config change: %v", err)
		}
	}

	// Return created route
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Route created successfully",
		"route":   route,
	})
}

// GetRoute handles GET /routes/{id}
func (rh *RouteHandler) GetRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract route ID from URL
	routeID := extractRouteID(r.URL.Path)
	if routeID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "Route ID is required", nil)
		return
	}

	ctx := context.Background()
	key := fmt.Sprintf("routes/%s", routeID)
	
	data, err := rh.store.Get(ctx, key)
	if err != nil {
		writeErrorResponse(w, http.StatusNotFound, "Route not found", err)
		return
	}

	var route router.RouteRule
	if err := json.Unmarshal(data, &route); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to deserialize route", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(route)
}

// UpdateRoute handles PUT /routes/{id}
func (rh *RouteHandler) UpdateRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract route ID from URL
	routeID := extractRouteID(r.URL.Path)
	if routeID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "Route ID is required", nil)
		return
	}

	var route router.RouteRule
	if err := json.NewDecoder(r.Body).Decode(&route); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON format", err)
		return
	}

	// Ensure ID matches URL
	route.ID = routeID

	// Update timestamp
	route.UpdatedAt = time.Now().Unix()

	// Validate route
	if err := route.Validate(); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Route validation failed", err)
		return
	}

	ctx := context.Background()
	key := fmt.Sprintf("routes/%s", routeID)

	// Get old route data for change notification
	oldData, err := rh.store.Get(ctx, key)
	if err != nil {
		writeErrorResponse(w, http.StatusNotFound, "Route not found", err)
		return
	}

	// Serialize route
	data, err := json.Marshal(route)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to serialize route", err)
		return
	}

	// Update route
	if err := rh.store.Put(ctx, key, data); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to update route", err)
		return
	}

	// Notify configuration change
	if rh.configNotifier != nil {
		if err := rh.configNotifier.PublishConfigChange("update", key, data, oldData, "admin_api"); err != nil {
			// Log error but don't fail the request
			log.Printf("Failed to publish config change: %v", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Route updated successfully",
		"route":   route,
	})
}

// DeleteRoute handles DELETE /routes/{id}
func (rh *RouteHandler) DeleteRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract route ID from URL
	routeID := extractRouteID(r.URL.Path)
	if routeID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "Route ID is required", nil)
		return
	}

	ctx := context.Background()
	key := fmt.Sprintf("routes/%s", routeID)

	// Get route data before deletion for change notification
	oldData, err := rh.store.Get(ctx, key)
	if err != nil {
		writeErrorResponse(w, http.StatusNotFound, "Route not found", err)
		return
	}

	// Delete route
	if err := rh.store.Delete(ctx, key); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to delete route", err)
		return
	}

	// Notify configuration change
	if rh.configNotifier != nil {
		if err := rh.configNotifier.PublishConfigChange("delete", key, nil, oldData, "admin_api"); err != nil {
			// Log error but don't fail the request
			log.Printf("Failed to publish config change: %v", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Route deleted successfully",
	})
}

// ListRoutes handles GET /routes
func (rh *RouteHandler) ListRoutes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := context.Background()
	
	// Get all routes
	routesData, err := rh.store.List(ctx, "routes/")
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to list routes", err)
		return
	}

	var routes []router.RouteRule
	for _, data := range routesData {
		var route router.RouteRule
		if err := json.Unmarshal(data, &route); err != nil {
			// Log error but continue with other routes
			continue
		}
		routes = append(routes, route)
	}

	// Parse query parameters for filtering and pagination
	query := r.URL.Query()
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
	total := len(routes)
	start := offset
	end := offset + limit
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	paginatedRoutes := routes[start:end]

	response := map[string]interface{}{
		"routes": paginatedRoutes,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Helper functions
func extractRouteID(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 2 && parts[0] == "routes" {
		return parts[1]
	}
	return ""
}

func generateRouteID() string {
	return fmt.Sprintf("route-%d", time.Now().UnixNano())
}

func writeErrorResponse(w http.ResponseWriter, statusCode int, message string, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	response := map[string]interface{}{
		"error":   message,
		"status":  statusCode,
	}
	
	if err != nil {
		response["details"] = err.Error()
	}
	
	json.NewEncoder(w).Encode(response)
}
