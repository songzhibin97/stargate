package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/portal/auth"
	"github.com/songzhibin97/stargate/internal/portal/gateway"
	"github.com/songzhibin97/stargate/internal/portal/middleware"
	"github.com/songzhibin97/stargate/pkg/portal"
)

// ApplicationHandler handles application-related API requests
type ApplicationHandler struct {
	config           *config.Config
	appRepo          portal.ApplicationRepository
	apiKeyGenerator  *auth.APIKeyGenerator
	appIDGenerator   *auth.ApplicationIDGenerator
	gatewayClient    GatewayClient
}

// GatewayClient defines the interface for interacting with the data plane gateway
type GatewayClient interface {
	CreateConsumer(consumerID, name string, metadata map[string]string) (*gateway.Consumer, error)
	DeleteConsumer(consumerID string) error
	GenerateAPIKey(consumerID string) (string, error)
	RevokeAPIKey(consumerID, apiKey string) error
	Health() error
}

// NewApplicationHandler creates a new application handler
func NewApplicationHandler(cfg *config.Config, appRepo portal.ApplicationRepository, gatewayClient GatewayClient) *ApplicationHandler {
	return &ApplicationHandler{
		config:          cfg,
		appRepo:         appRepo,
		apiKeyGenerator: auth.NewAPIKeyGenerator(),
		appIDGenerator:  auth.NewApplicationIDGenerator(),
		gatewayClient:   gatewayClient,
	}
}

// CreateApplicationRequest represents a request to create an application
type CreateApplicationRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// UpdateApplicationRequest represents a request to update an application
type UpdateApplicationRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// ApplicationResponse represents an application response
type ApplicationResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	UserID      string    `json:"user_id"`
	APIKey      string    `json:"api_key"`
	Status      string    `json:"status"`
	RateLimit   int64     `json:"rate_limit"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ApplicationListResponse represents a paginated list of applications
type ApplicationListResponse struct {
	Applications []*ApplicationResponse `json:"applications"`
	Total        int64                  `json:"total"`
	Offset       int                    `json:"offset"`
	Limit        int                    `json:"limit"`
	HasMore      bool                   `json:"has_more"`
}

// HandleCreateApplication handles POST /api/applications
func (ah *ApplicationHandler) HandleCreateApplication(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ah.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	// Get user ID from JWT context
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		ah.writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated")
		return
	}

	// Parse request
	var req CreateApplicationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ah.writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON format")
		return
	}

	// Validate request
	if err := ah.validateCreateRequest(&req); err != nil {
		ah.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	ctx := r.Context()

	// Generate application ID
	appID, err := ah.appIDGenerator.GenerateApplicationID()
	if err != nil {
		ah.writeError(w, http.StatusInternalServerError, "ID_GENERATION_ERROR", "Failed to generate application ID")
		return
	}

	// Generate API key
	apiKey, err := ah.apiKeyGenerator.GenerateAPIKey("app")
	if err != nil {
		ah.writeError(w, http.StatusInternalServerError, "API_KEY_GENERATION_ERROR", "Failed to generate API key")
		return
	}

	// Generate API secret
	apiSecret, err := ah.apiKeyGenerator.GenerateAPIKey("secret")
	if err != nil {
		ah.writeError(w, http.StatusInternalServerError, "API_SECRET_GENERATION_ERROR", "Failed to generate API secret")
		return
	}

	// Create application
	app := &portal.Application{
		ID:          appID,
		Name:        req.Name,
		Description: req.Description,
		UserID:      userID,
		APIKey:      apiKey,
		APISecret:   apiSecret,
		Status:      portal.ApplicationStatusActive,
		RateLimit:   1000, // Default rate limit
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Create consumer in gateway
	consumer, err := ah.gatewayClient.CreateConsumer(appID, req.Name, map[string]string{
		"user_id":     userID,
		"app_id":      appID,
		"created_by":  "portal",
	})
	if err != nil {
		ah.writeError(w, http.StatusInternalServerError, "GATEWAY_ERROR", "Failed to create consumer in gateway")
		return
	}

	// Use the API key from gateway if provided
	if consumer.APIKey != "" {
		app.APIKey = consumer.APIKey
	}

	// Save application to database
	if err := ah.appRepo.CreateApplication(ctx, app); err != nil {
		// Cleanup: delete consumer from gateway if application creation fails
		ah.gatewayClient.DeleteConsumer(appID)
		
		if portal.IsConflictError(err) {
			ah.writeError(w, http.StatusConflict, "APPLICATION_EXISTS", "Application with this name already exists")
		} else {
			ah.writeError(w, http.StatusInternalServerError, "CREATE_ERROR", "Failed to create application")
		}
		return
	}

	// Prepare response
	response := ah.toApplicationResponse(app)
	ah.writeJSON(w, http.StatusCreated, response)
}

// HandleGetApplication handles GET /api/applications/{id}
func (ah *ApplicationHandler) HandleGetApplication(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		ah.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	// Get user ID from JWT context
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		ah.writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated")
		return
	}

	// Extract application ID from URL
	appID := ah.extractIDFromPath(r.URL.Path, "/api/applications/")
	if appID == "" {
		ah.writeError(w, http.StatusBadRequest, "INVALID_APPLICATION_ID", "Application ID is required")
		return
	}

	ctx := r.Context()

	// Get application
	app, err := ah.appRepo.GetApplication(ctx, appID)
	if err != nil {
		if portal.IsNotFoundError(err) {
			ah.writeError(w, http.StatusNotFound, "APPLICATION_NOT_FOUND", "Application not found")
		} else {
			ah.writeError(w, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to retrieve application")
		}
		return
	}

	// Check if user owns the application
	if app.UserID != userID {
		ah.writeError(w, http.StatusForbidden, "ACCESS_DENIED", "You don't have access to this application")
		return
	}

	// Prepare response
	response := ah.toApplicationResponse(app)
	ah.writeJSON(w, http.StatusOK, response)
}

// HandleListApplications handles GET /api/applications
func (ah *ApplicationHandler) HandleListApplications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		ah.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	// Get user ID from JWT context
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		ah.writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated")
		return
	}

	// Parse query parameters
	offset, limit := ah.parsePaginationParams(r)
	
	ctx := r.Context()

	// Create filter for user's applications
	filter := &portal.ApplicationFilter{
		UserID: userID,
		Offset: offset,
		Limit:  limit,
		SortBy: "created_at",
		SortOrder: "desc",
	}

	// Get applications
	result, err := ah.appRepo.ListApplications(ctx, filter)
	if err != nil {
		ah.writeError(w, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to retrieve applications")
		return
	}

	// Convert to response format
	applications := make([]*ApplicationResponse, len(result.Applications))
	for i, app := range result.Applications {
		applications[i] = ah.toApplicationResponse(app)
	}

	response := &ApplicationListResponse{
		Applications: applications,
		Total:        result.Total,
		Offset:       result.Offset,
		Limit:        result.Limit,
		HasMore:      result.HasMore,
	}

	ah.writeJSON(w, http.StatusOK, response)
}

// HandleUpdateApplication handles PUT /api/applications/{id}
func (ah *ApplicationHandler) HandleUpdateApplication(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		ah.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	// Get user ID from JWT context
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		ah.writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated")
		return
	}

	// Extract application ID from URL
	appID := ah.extractIDFromPath(r.URL.Path, "/api/applications/")
	if appID == "" {
		ah.writeError(w, http.StatusBadRequest, "INVALID_APPLICATION_ID", "Application ID is required")
		return
	}

	// Parse request
	var req UpdateApplicationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ah.writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON format")
		return
	}

	ctx := r.Context()

	// Get existing application
	app, err := ah.appRepo.GetApplication(ctx, appID)
	if err != nil {
		if portal.IsNotFoundError(err) {
			ah.writeError(w, http.StatusNotFound, "APPLICATION_NOT_FOUND", "Application not found")
		} else {
			ah.writeError(w, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to retrieve application")
		}
		return
	}

	// Check if user owns the application
	if app.UserID != userID {
		ah.writeError(w, http.StatusForbidden, "ACCESS_DENIED", "You don't have access to this application")
		return
	}

	// Update fields
	if req.Name != "" {
		app.Name = req.Name
	}
	if req.Description != "" {
		app.Description = req.Description
	}
	app.UpdatedAt = time.Now()

	// Validate updated application
	if err := ah.validateUpdateRequest(&req); err != nil {
		ah.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	// Update application
	if err := ah.appRepo.UpdateApplication(ctx, app); err != nil {
		ah.writeError(w, http.StatusInternalServerError, "UPDATE_ERROR", "Failed to update application")
		return
	}

	// Prepare response
	response := ah.toApplicationResponse(app)
	ah.writeJSON(w, http.StatusOK, response)
}

// HandleDeleteApplication handles DELETE /api/applications/{id}
func (ah *ApplicationHandler) HandleDeleteApplication(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		ah.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	// Get user ID from JWT context
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		ah.writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated")
		return
	}

	// Extract application ID from URL
	appID := ah.extractIDFromPath(r.URL.Path, "/api/applications/")
	if appID == "" {
		ah.writeError(w, http.StatusBadRequest, "INVALID_APPLICATION_ID", "Application ID is required")
		return
	}

	ctx := r.Context()

	// Get existing application
	app, err := ah.appRepo.GetApplication(ctx, appID)
	if err != nil {
		if portal.IsNotFoundError(err) {
			ah.writeError(w, http.StatusNotFound, "APPLICATION_NOT_FOUND", "Application not found")
		} else {
			ah.writeError(w, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to retrieve application")
		}
		return
	}

	// Check if user owns the application
	if app.UserID != userID {
		ah.writeError(w, http.StatusForbidden, "ACCESS_DENIED", "You don't have access to this application")
		return
	}

	// Delete consumer from gateway
	if err := ah.gatewayClient.DeleteConsumer(appID); err != nil {
		// Log error but continue with application deletion
		// Gateway consumer deletion failure shouldn't block application deletion
	}

	// Delete application
	if err := ah.appRepo.DeleteApplication(ctx, appID); err != nil {
		ah.writeError(w, http.StatusInternalServerError, "DELETE_ERROR", "Failed to delete application")
		return
	}

	// Return success response
	response := map[string]interface{}{
		"message": "Application deleted successfully",
		"id":      appID,
	}
	ah.writeJSON(w, http.StatusOK, response)
}

// HandleRegenerateAPIKey handles POST /api/applications/{id}/regenerate-key
func (ah *ApplicationHandler) HandleRegenerateAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ah.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	// Get user ID from JWT context
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		ah.writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated")
		return
	}

	// Extract application ID from URL
	appID := ah.extractIDFromPath(r.URL.Path, "/api/applications/")
	if appID == "" {
		ah.writeError(w, http.StatusBadRequest, "INVALID_APPLICATION_ID", "Application ID is required")
		return
	}

	ctx := r.Context()

	// Get existing application
	app, err := ah.appRepo.GetApplication(ctx, appID)
	if err != nil {
		if portal.IsNotFoundError(err) {
			ah.writeError(w, http.StatusNotFound, "APPLICATION_NOT_FOUND", "Application not found")
		} else {
			ah.writeError(w, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to retrieve application")
		}
		return
	}

	// Check if user owns the application
	if app.UserID != userID {
		ah.writeError(w, http.StatusForbidden, "ACCESS_DENIED", "You don't have access to this application")
		return
	}

	// Regenerate API key
	newAPIKey, err := ah.appRepo.RegenerateAPIKey(ctx, appID)
	if err != nil {
		ah.writeError(w, http.StatusInternalServerError, "REGENERATE_ERROR", "Failed to regenerate API key")
		return
	}

	// Update gateway consumer with new API key
	if _, err := ah.gatewayClient.GenerateAPIKey(appID); err != nil {
		// Log error but continue - the database has been updated
	}

	// Return new API key
	response := map[string]interface{}{
		"message": "API key regenerated successfully",
		"api_key": newAPIKey,
	}
	ah.writeJSON(w, http.StatusOK, response)
}

// Helper methods

// validateCreateRequest validates a create application request
func (ah *ApplicationHandler) validateCreateRequest(req *CreateApplicationRequest) error {
	if req.Name == "" {
		return fmt.Errorf("application name is required")
	}
	if len(req.Name) > 255 {
		return fmt.Errorf("application name must be less than 255 characters")
	}
	if len(req.Description) > 1000 {
		return fmt.Errorf("application description must be less than 1000 characters")
	}
	return nil
}

// validateUpdateRequest validates an update application request
func (ah *ApplicationHandler) validateUpdateRequest(req *UpdateApplicationRequest) error {
	if req.Name != "" && len(req.Name) > 255 {
		return fmt.Errorf("application name must be less than 255 characters")
	}
	if req.Description != "" && len(req.Description) > 1000 {
		return fmt.Errorf("application description must be less than 1000 characters")
	}
	return nil
}

// extractIDFromPath extracts ID from URL path
func (ah *ApplicationHandler) extractIDFromPath(path, prefix string) string {
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	remainder := strings.TrimPrefix(path, prefix)
	parts := strings.Split(remainder, "/")
	if len(parts) == 0 || parts[0] == "" {
		return ""
	}
	return parts[0]
}

// parsePaginationParams parses pagination parameters from request
func (ah *ApplicationHandler) parsePaginationParams(r *http.Request) (offset, limit int) {
	offsetStr := r.URL.Query().Get("offset")
	limitStr := r.URL.Query().Get("limit")

	offset = 0
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	limit = 20 // Default limit
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	return offset, limit
}

// toApplicationResponse converts Application to ApplicationResponse
func (ah *ApplicationHandler) toApplicationResponse(app *portal.Application) *ApplicationResponse {
	return &ApplicationResponse{
		ID:          app.ID,
		Name:        app.Name,
		Description: app.Description,
		UserID:      app.UserID,
		APIKey:      app.APIKey,
		Status:      string(app.Status),
		RateLimit:   app.RateLimit,
		CreatedAt:   app.CreatedAt,
		UpdatedAt:   app.UpdatedAt,
	}
}

// writeJSON writes a JSON response
func (ah *ApplicationHandler) writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// writeError writes an error response
func (ah *ApplicationHandler) writeError(w http.ResponseWriter, statusCode int, code, message string) {
	response := map[string]interface{}{
		"error":   http.StatusText(statusCode),
		"message": message,
		"code":    code,
	}
	ah.writeJSON(w, statusCode, response)
}
