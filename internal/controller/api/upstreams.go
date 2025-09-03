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
	"github.com/songzhibin97/stargate/internal/router"
	"github.com/songzhibin97/stargate/internal/store"
)

// UpstreamHandler handles upstream management API requests
type UpstreamHandler struct {
	config         *config.Config
	store          store.Store
	configNotifier ConfigNotifier
}

// NewUpstreamHandler creates a new upstream handler
func NewUpstreamHandler(cfg *config.Config, store store.Store, configNotifier ConfigNotifier) *UpstreamHandler {
	return &UpstreamHandler{
		config:         cfg,
		store:          store,
		configNotifier: configNotifier,
	}
}

// CreateUpstream handles POST /upstreams
func (uh *UpstreamHandler) CreateUpstream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var upstream router.Upstream
	if err := json.NewDecoder(r.Body).Decode(&upstream); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON format", err)
		return
	}

	// Generate ID if not provided
	if upstream.ID == "" {
		upstream.ID = generateUpstreamID()
	}

	// Set timestamps
	upstream.SetTimestamps()

	// Validate upstream
	if err := upstream.Validate(); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Upstream validation failed", err)
		return
	}

	// Check if upstream ID already exists
	ctx := context.Background()
	key := fmt.Sprintf("upstreams/%s", upstream.ID)
	if _, err := uh.store.Get(ctx, key); err == nil {
		writeErrorResponse(w, http.StatusConflict, "Upstream ID already exists", nil)
		return
	}

	// Serialize upstream
	data, err := json.Marshal(upstream)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to serialize upstream", err)
		return
	}

	// Store upstream
	if err := uh.store.Put(ctx, key, data); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to store upstream", err)
		return
	}

	// Return created upstream
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":  "Upstream created successfully",
		"upstream": upstream,
	})
}

// GetUpstream handles GET /upstreams/{id}
func (uh *UpstreamHandler) GetUpstream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract upstream ID from URL
	upstreamID := extractUpstreamID(r.URL.Path)
	if upstreamID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "Upstream ID is required", nil)
		return
	}

	ctx := context.Background()
	key := fmt.Sprintf("upstreams/%s", upstreamID)
	
	data, err := uh.store.Get(ctx, key)
	if err != nil {
		writeErrorResponse(w, http.StatusNotFound, "Upstream not found", err)
		return
	}

	var upstream router.Upstream
	if err := json.Unmarshal(data, &upstream); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to deserialize upstream", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(upstream)
}

// UpdateUpstream handles PUT /upstreams/{id}
func (uh *UpstreamHandler) UpdateUpstream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract upstream ID from URL
	upstreamID := extractUpstreamID(r.URL.Path)
	if upstreamID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "Upstream ID is required", nil)
		return
	}

	var upstream router.Upstream
	if err := json.NewDecoder(r.Body).Decode(&upstream); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON format", err)
		return
	}

	// Ensure ID matches URL
	upstream.ID = upstreamID

	// Update timestamp
	upstream.UpdatedAt = time.Now().Unix()

	// Validate upstream
	if err := upstream.Validate(); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Upstream validation failed", err)
		return
	}

	ctx := context.Background()
	key := fmt.Sprintf("upstreams/%s", upstreamID)

	// Check if upstream exists
	if _, err := uh.store.Get(ctx, key); err != nil {
		writeErrorResponse(w, http.StatusNotFound, "Upstream not found", err)
		return
	}

	// Serialize upstream
	data, err := json.Marshal(upstream)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to serialize upstream", err)
		return
	}

	// Update upstream
	if err := uh.store.Put(ctx, key, data); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to update upstream", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":  "Upstream updated successfully",
		"upstream": upstream,
	})
}

// DeleteUpstream handles DELETE /upstreams/{id}
func (uh *UpstreamHandler) DeleteUpstream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract upstream ID from URL
	upstreamID := extractUpstreamID(r.URL.Path)
	if upstreamID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "Upstream ID is required", nil)
		return
	}

	ctx := context.Background()
	key := fmt.Sprintf("upstreams/%s", upstreamID)

	// Check if upstream exists
	if _, err := uh.store.Get(ctx, key); err != nil {
		writeErrorResponse(w, http.StatusNotFound, "Upstream not found", err)
		return
	}

	// Check if upstream is referenced by any routes
	if err := uh.checkUpstreamReferences(ctx, upstreamID); err != nil {
		writeErrorResponse(w, http.StatusConflict, "Upstream is referenced by routes", err)
		return
	}

	// Delete upstream
	if err := uh.store.Delete(ctx, key); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to delete upstream", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Upstream deleted successfully",
	})
}

// ListUpstreams handles GET /upstreams
func (uh *UpstreamHandler) ListUpstreams(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := context.Background()
	
	// Get all upstreams
	upstreamsData, err := uh.store.List(ctx, "upstreams/")
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to list upstreams", err)
		return
	}

	var upstreams []router.Upstream
	for _, data := range upstreamsData {
		var upstream router.Upstream
		if err := json.Unmarshal(data, &upstream); err != nil {
			// Log error but continue with other upstreams
			continue
		}
		upstreams = append(upstreams, upstream)
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
	total := len(upstreams)
	start := offset
	end := offset + limit
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	paginatedUpstreams := upstreams[start:end]

	response := map[string]interface{}{
		"upstreams": paginatedUpstreams,
		"total":     total,
		"limit":     limit,
		"offset":    offset,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// checkUpstreamReferences checks if upstream is referenced by any routes
func (uh *UpstreamHandler) checkUpstreamReferences(ctx context.Context, upstreamID string) error {
	routesData, err := uh.store.List(ctx, "routes/")
	if err != nil {
		return err
	}

	for _, data := range routesData {
		var route router.RouteRule
		if err := json.Unmarshal(data, &route); err != nil {
			continue
		}
		if route.UpstreamID == upstreamID {
			return fmt.Errorf("upstream is referenced by route %s", route.ID)
		}
	}

	return nil
}

// Helper functions
func extractUpstreamID(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 3 && parts[len(parts)-2] == "upstreams" {
		return parts[len(parts)-1]
	}
	return ""
}

func generateUpstreamID() string {
	return fmt.Sprintf("upstream-%d", time.Now().UnixNano())
}
