package memory

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sort"
	"strings"
	"time"

	"github.com/songzhibin97/stargate/pkg/portal"
)

// ApplicationRepository implements the portal.ApplicationRepository interface using in-memory storage
type ApplicationRepository struct {
	repo *Repository
	tx   *Transaction
}

// NewApplicationRepository creates a new in-memory application repository
func NewApplicationRepository(repo *Repository) *ApplicationRepository {
	return &ApplicationRepository{
		repo: repo,
	}
}

// CreateApplication creates a new application
func (ar *ApplicationRepository) CreateApplication(ctx context.Context, app *portal.Application) error {
	if ar.tx != nil {
		if err := ar.tx.isActive(); err != nil {
			return err
		}
	}

	ar.repo.mu.Lock()
	defer ar.repo.mu.Unlock()

	if ar.repo.closed {
		return portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if err := ar.repo.isValidApplication(app); err != nil {
		return err
	}

	// Check if application already exists
	if _, exists := ar.repo.applications[app.ID]; exists {
		return portal.NewConflictError("APPLICATION_ALREADY_EXISTS", "application with this ID already exists")
	}

	// Check if API key already exists
	if _, exists := ar.repo.appsByAPIKey[app.APIKey]; exists {
		return portal.NewConflictError("APPLICATION_API_KEY_EXISTS", "application with this API key already exists")
	}

	// Verify user exists
	if _, exists := ar.repo.users[app.UserID]; !exists {
		return portal.NewNotFoundError("USER_NOT_FOUND", "user not found")
	}

	// Set timestamps
	now := time.Now()
	if app.CreatedAt.IsZero() {
		app.CreatedAt = now
	}
	app.UpdatedAt = now

	// Create a copy to avoid external modifications
	appCopy := *app
	ar.repo.applications[app.ID] = &appCopy
	ar.repo.addApplicationToIndex(&appCopy)

	return nil
}

// GetApplication retrieves an application by ID
func (ar *ApplicationRepository) GetApplication(ctx context.Context, appID string) (*portal.Application, error) {
	if ar.tx != nil {
		if err := ar.tx.isActive(); err != nil {
			return nil, err
		}
	}

	ar.repo.mu.RLock()
	defer ar.repo.mu.RUnlock()

	if ar.repo.closed {
		return nil, portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if appID == "" {
		return nil, portal.NewValidationError("INVALID_APPLICATION_ID", "application ID cannot be empty")
	}

	app, exists := ar.repo.applications[appID]
	if !exists {
		return nil, portal.NewNotFoundError("APPLICATION_NOT_FOUND", "application not found")
	}

	// Return a copy to avoid external modifications
	appCopy := *app
	return &appCopy, nil
}

// GetApplicationByAPIKey retrieves an application by API key
func (ar *ApplicationRepository) GetApplicationByAPIKey(ctx context.Context, apiKey string) (*portal.Application, error) {
	if ar.tx != nil {
		if err := ar.tx.isActive(); err != nil {
			return nil, err
		}
	}

	ar.repo.mu.RLock()
	defer ar.repo.mu.RUnlock()

	if ar.repo.closed {
		return nil, portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if apiKey == "" {
		return nil, portal.NewValidationError("INVALID_API_KEY", "API key cannot be empty")
	}

	app, exists := ar.repo.appsByAPIKey[apiKey]
	if !exists {
		return nil, portal.NewNotFoundError("APPLICATION_NOT_FOUND", "application not found")
	}

	// Return a copy to avoid external modifications
	appCopy := *app
	return &appCopy, nil
}

// GetApplicationsByUser retrieves all applications for a specific user
func (ar *ApplicationRepository) GetApplicationsByUser(ctx context.Context, userID string) ([]*portal.Application, error) {
	if ar.tx != nil {
		if err := ar.tx.isActive(); err != nil {
			return nil, err
		}
	}

	ar.repo.mu.RLock()
	defer ar.repo.mu.RUnlock()

	if ar.repo.closed {
		return nil, portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if userID == "" {
		return nil, portal.NewValidationError("INVALID_USER_ID", "user ID cannot be empty")
	}

	apps, exists := ar.repo.appsByUser[userID]
	if !exists {
		return []*portal.Application{}, nil
	}

	// Return copies to avoid external modifications
	result := make([]*portal.Application, len(apps))
	for i, app := range apps {
		appCopy := *app
		result[i] = &appCopy
	}

	return result, nil
}

// UpdateApplication updates an existing application
func (ar *ApplicationRepository) UpdateApplication(ctx context.Context, app *portal.Application) error {
	if ar.tx != nil {
		if err := ar.tx.isActive(); err != nil {
			return err
		}
	}

	ar.repo.mu.Lock()
	defer ar.repo.mu.Unlock()

	if ar.repo.closed {
		return portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if err := ar.repo.isValidApplication(app); err != nil {
		return err
	}

	// Check if application exists
	existingApp, exists := ar.repo.applications[app.ID]
	if !exists {
		return portal.NewNotFoundError("APPLICATION_NOT_FOUND", "application not found")
	}

	// Check if API key is being changed and if new API key already exists
	if existingApp.APIKey != app.APIKey {
		if _, apiKeyExists := ar.repo.appsByAPIKey[app.APIKey]; apiKeyExists {
			return portal.NewConflictError("APPLICATION_API_KEY_EXISTS", "application with this API key already exists")
		}
		// Remove old API key from index
		ar.repo.removeApplicationFromIndex(existingApp)
	}

	// Verify user exists
	if _, exists := ar.repo.users[app.UserID]; !exists {
		return portal.NewNotFoundError("USER_NOT_FOUND", "user not found")
	}

	// Update timestamps
	app.CreatedAt = existingApp.CreatedAt // Preserve original creation time
	app.UpdatedAt = time.Now()

	// Create a copy and update
	appCopy := *app
	ar.repo.applications[app.ID] = &appCopy
	ar.repo.addApplicationToIndex(&appCopy)

	return nil
}

// DeleteApplication deletes an application by ID
func (ar *ApplicationRepository) DeleteApplication(ctx context.Context, appID string) error {
	if ar.tx != nil {
		if err := ar.tx.isActive(); err != nil {
			return err
		}
	}

	ar.repo.mu.Lock()
	defer ar.repo.mu.Unlock()

	if ar.repo.closed {
		return portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if appID == "" {
		return portal.NewValidationError("INVALID_APPLICATION_ID", "application ID cannot be empty")
	}

	app, exists := ar.repo.applications[appID]
	if !exists {
		return portal.NewNotFoundError("APPLICATION_NOT_FOUND", "application not found")
	}

	// Remove from indexes
	ar.repo.removeApplicationFromIndex(app)
	delete(ar.repo.applications, appID)

	return nil
}

// ExistsApplication checks if an application exists by ID
func (ar *ApplicationRepository) ExistsApplication(ctx context.Context, appID string) (bool, error) {
	if ar.tx != nil {
		if err := ar.tx.isActive(); err != nil {
			return false, err
		}
	}

	ar.repo.mu.RLock()
	defer ar.repo.mu.RUnlock()

	if ar.repo.closed {
		return false, portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if appID == "" {
		return false, portal.NewValidationError("INVALID_APPLICATION_ID", "application ID cannot be empty")
	}

	_, exists := ar.repo.applications[appID]
	return exists, nil
}

// ExistsApplicationByAPIKey checks if an application exists by API key
func (ar *ApplicationRepository) ExistsApplicationByAPIKey(ctx context.Context, apiKey string) (bool, error) {
	if ar.tx != nil {
		if err := ar.tx.isActive(); err != nil {
			return false, err
		}
	}

	ar.repo.mu.RLock()
	defer ar.repo.mu.RUnlock()

	if ar.repo.closed {
		return false, portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if apiKey == "" {
		return false, portal.NewValidationError("INVALID_API_KEY", "API key cannot be empty")
	}

	_, exists := ar.repo.appsByAPIKey[apiKey]
	return exists, nil
}

// UpdateApplicationStatus updates the status of an application
func (ar *ApplicationRepository) UpdateApplicationStatus(ctx context.Context, appID string, status portal.ApplicationStatus) error {
	if ar.tx != nil {
		if err := ar.tx.isActive(); err != nil {
			return err
		}
	}

	ar.repo.mu.Lock()
	defer ar.repo.mu.Unlock()

	if ar.repo.closed {
		return portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if appID == "" {
		return portal.NewValidationError("INVALID_APPLICATION_ID", "application ID cannot be empty")
	}

	app, exists := ar.repo.applications[appID]
	if !exists {
		return portal.NewNotFoundError("APPLICATION_NOT_FOUND", "application not found")
	}

	// Update status
	app.Status = status
	app.UpdatedAt = time.Now()

	return nil
}

// UpdateApplicationRateLimit updates the rate limit of an application
func (ar *ApplicationRepository) UpdateApplicationRateLimit(ctx context.Context, appID string, rateLimit int64) error {
	if ar.tx != nil {
		if err := ar.tx.isActive(); err != nil {
			return err
		}
	}

	ar.repo.mu.Lock()
	defer ar.repo.mu.Unlock()

	if ar.repo.closed {
		return portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if appID == "" {
		return portal.NewValidationError("INVALID_APPLICATION_ID", "application ID cannot be empty")
	}

	if rateLimit < 0 {
		return portal.NewValidationError("INVALID_RATE_LIMIT", "rate limit cannot be negative")
	}

	app, exists := ar.repo.applications[appID]
	if !exists {
		return portal.NewNotFoundError("APPLICATION_NOT_FOUND", "application not found")
	}

	// Update rate limit
	app.RateLimit = rateLimit
	app.UpdatedAt = time.Now()

	return nil
}

// RegenerateAPIKey generates a new API key for an application
func (ar *ApplicationRepository) RegenerateAPIKey(ctx context.Context, appID string) (string, error) {
	if ar.tx != nil {
		if err := ar.tx.isActive(); err != nil {
			return "", err
		}
	}

	ar.repo.mu.Lock()
	defer ar.repo.mu.Unlock()

	if ar.repo.closed {
		return "", portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if appID == "" {
		return "", portal.NewValidationError("INVALID_APPLICATION_ID", "application ID cannot be empty")
	}

	app, exists := ar.repo.applications[appID]
	if !exists {
		return "", portal.NewNotFoundError("APPLICATION_NOT_FOUND", "application not found")
	}

	// Remove old API key from index
	delete(ar.repo.appsByAPIKey, app.APIKey)

	// Generate new API key
	newAPIKey, err := ar.generateAPIKey()
	if err != nil {
		return "", portal.NewInternalError("API_KEY_GENERATION_FAILED", "failed to generate API key", err)
	}

	// Update application
	app.APIKey = newAPIKey
	app.UpdatedAt = time.Now()

	// Add new API key to index
	ar.repo.appsByAPIKey[newAPIKey] = app

	return newAPIKey, nil
}

// RegenerateAPISecret generates a new API secret for an application
func (ar *ApplicationRepository) RegenerateAPISecret(ctx context.Context, appID string) (string, error) {
	if ar.tx != nil {
		if err := ar.tx.isActive(); err != nil {
			return "", err
		}
	}

	ar.repo.mu.Lock()
	defer ar.repo.mu.Unlock()

	if ar.repo.closed {
		return "", portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if appID == "" {
		return "", portal.NewValidationError("INVALID_APPLICATION_ID", "application ID cannot be empty")
	}

	app, exists := ar.repo.applications[appID]
	if !exists {
		return "", portal.NewNotFoundError("APPLICATION_NOT_FOUND", "application not found")
	}

	// Generate new API secret
	newAPISecret, err := ar.generateAPISecret()
	if err != nil {
		return "", portal.NewInternalError("API_SECRET_GENERATION_FAILED", "failed to generate API secret", err)
	}

	// Update application
	app.APISecret = newAPISecret
	app.UpdatedAt = time.Now()

	return newAPISecret, nil
}

// CountApplicationsByUser returns the count of applications for a specific user
func (ar *ApplicationRepository) CountApplicationsByUser(ctx context.Context, userID string) (int64, error) {
	if ar.tx != nil {
		if err := ar.tx.isActive(); err != nil {
			return 0, err
		}
	}

	ar.repo.mu.RLock()
	defer ar.repo.mu.RUnlock()

	if ar.repo.closed {
		return 0, portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if userID == "" {
		return 0, portal.NewValidationError("INVALID_USER_ID", "user ID cannot be empty")
	}

	apps, exists := ar.repo.appsByUser[userID]
	if !exists {
		return 0, nil
	}

	return int64(len(apps)), nil
}

// ListApplications retrieves applications based on filter criteria
func (ar *ApplicationRepository) ListApplications(ctx context.Context, filter *portal.ApplicationFilter) (*portal.PaginatedApplications, error) {
	if ar.tx != nil {
		if err := ar.tx.isActive(); err != nil {
			return nil, err
		}
	}

	ar.repo.mu.RLock()
	defer ar.repo.mu.RUnlock()

	if ar.repo.closed {
		return nil, portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if filter == nil {
		filter = &portal.ApplicationFilter{}
	}

	// Set default values
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	if filter.SortBy == "" {
		filter.SortBy = "created_at"
	}
	if filter.SortOrder == "" {
		filter.SortOrder = "desc"
	}

	// Collect all applications that match the filter
	var filteredApps []*portal.Application
	for _, app := range ar.repo.applications {
		if ar.matchesApplicationFilter(app, filter) {
			appCopy := *app
			filteredApps = append(filteredApps, &appCopy)
		}
	}

	// Sort applications
	ar.sortApplications(filteredApps, filter.SortBy, filter.SortOrder)

	// Calculate pagination
	total := int64(len(filteredApps))
	start := filter.Offset
	end := start + filter.Limit

	if start > len(filteredApps) {
		start = len(filteredApps)
	}
	if end > len(filteredApps) {
		end = len(filteredApps)
	}

	paginatedApps := filteredApps[start:end]
	hasMore := end < len(filteredApps)

	return &portal.PaginatedApplications{
		Applications: paginatedApps,
		Total:        total,
		Offset:       filter.Offset,
		Limit:        filter.Limit,
		HasMore:      hasMore,
	}, nil
}

// CountApplications returns the total count of applications matching the filter
func (ar *ApplicationRepository) CountApplications(ctx context.Context, filter *portal.ApplicationFilter) (int64, error) {
	if ar.tx != nil {
		if err := ar.tx.isActive(); err != nil {
			return 0, err
		}
	}

	ar.repo.mu.RLock()
	defer ar.repo.mu.RUnlock()

	if ar.repo.closed {
		return 0, portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if filter == nil {
		return int64(len(ar.repo.applications)), nil
	}

	count := int64(0)
	for _, app := range ar.repo.applications {
		if ar.matchesApplicationFilter(app, filter) {
			count++
		}
	}

	return count, nil
}

// BatchCreateApplications creates multiple applications in a single operation
func (ar *ApplicationRepository) BatchCreateApplications(ctx context.Context, apps []*portal.Application) error {
	if ar.tx != nil {
		if err := ar.tx.isActive(); err != nil {
			return err
		}
	}

	ar.repo.mu.Lock()
	defer ar.repo.mu.Unlock()

	if ar.repo.closed {
		return portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if len(apps) == 0 {
		return nil
	}

	// Validate all applications first
	for _, app := range apps {
		if err := ar.repo.isValidApplication(app); err != nil {
			return err
		}

		// Check if application already exists
		if _, exists := ar.repo.applications[app.ID]; exists {
			return portal.NewConflictError("APPLICATION_ALREADY_EXISTS", "application with ID "+app.ID+" already exists")
		}

		// Check if API key already exists
		if _, exists := ar.repo.appsByAPIKey[app.APIKey]; exists {
			return portal.NewConflictError("APPLICATION_API_KEY_EXISTS", "application with API key "+app.APIKey+" already exists")
		}

		// Verify user exists
		if _, exists := ar.repo.users[app.UserID]; !exists {
			return portal.NewNotFoundError("USER_NOT_FOUND", "user with ID "+app.UserID+" not found")
		}
	}

	// Create all applications
	now := time.Now()
	for _, app := range apps {
		// Set timestamps
		if app.CreatedAt.IsZero() {
			app.CreatedAt = now
		}
		app.UpdatedAt = now

		// Create a copy to avoid external modifications
		appCopy := *app
		ar.repo.applications[app.ID] = &appCopy
		ar.repo.addApplicationToIndex(&appCopy)
	}

	return nil
}

// BatchUpdateApplications updates multiple applications in a single operation
func (ar *ApplicationRepository) BatchUpdateApplications(ctx context.Context, apps []*portal.Application) error {
	if ar.tx != nil {
		if err := ar.tx.isActive(); err != nil {
			return err
		}
	}

	ar.repo.mu.Lock()
	defer ar.repo.mu.Unlock()

	if ar.repo.closed {
		return portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if len(apps) == 0 {
		return nil
	}

	// Validate all applications first and check existence
	for _, app := range apps {
		if err := ar.repo.isValidApplication(app); err != nil {
			return err
		}

		// Check if application exists
		existingApp, exists := ar.repo.applications[app.ID]
		if !exists {
			return portal.NewNotFoundError("APPLICATION_NOT_FOUND", "application with ID "+app.ID+" not found")
		}

		// Check if API key is being changed and if new API key already exists
		if existingApp.APIKey != app.APIKey {
			if _, apiKeyExists := ar.repo.appsByAPIKey[app.APIKey]; apiKeyExists {
				return portal.NewConflictError("APPLICATION_API_KEY_EXISTS", "application with API key "+app.APIKey+" already exists")
			}
		}

		// Verify user exists
		if _, exists := ar.repo.users[app.UserID]; !exists {
			return portal.NewNotFoundError("USER_NOT_FOUND", "user with ID "+app.UserID+" not found")
		}
	}

	// Update all applications
	now := time.Now()
	for _, app := range apps {
		existingApp := ar.repo.applications[app.ID]

		// Remove old API key from index if changed
		if existingApp.APIKey != app.APIKey {
			ar.repo.removeApplicationFromIndex(existingApp)
		}

		// Update timestamps
		app.CreatedAt = existingApp.CreatedAt // Preserve original creation time
		app.UpdatedAt = now

		// Create a copy and update
		appCopy := *app
		ar.repo.applications[app.ID] = &appCopy
		ar.repo.addApplicationToIndex(&appCopy)
	}

	return nil
}

// BatchDeleteApplications deletes multiple applications by IDs
func (ar *ApplicationRepository) BatchDeleteApplications(ctx context.Context, appIDs []string) error {
	if ar.tx != nil {
		if err := ar.tx.isActive(); err != nil {
			return err
		}
	}

	ar.repo.mu.Lock()
	defer ar.repo.mu.Unlock()

	if ar.repo.closed {
		return portal.NewDatabaseError("REPO_CLOSED", "repository is closed", nil)
	}

	if len(appIDs) == 0 {
		return nil
	}

	// Validate all application IDs first
	for _, appID := range appIDs {
		if appID == "" {
			return portal.NewValidationError("INVALID_APPLICATION_ID", "application ID cannot be empty")
		}

		if _, exists := ar.repo.applications[appID]; !exists {
			return portal.NewNotFoundError("APPLICATION_NOT_FOUND", "application with ID "+appID+" not found")
		}
	}

	// Delete all applications
	for _, appID := range appIDs {
		app := ar.repo.applications[appID]
		ar.repo.removeApplicationFromIndex(app)
		delete(ar.repo.applications, appID)
	}

	return nil
}

// generateAPIKey generates a new API key
func (ar *ApplicationRepository) generateAPIKey() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "ak_" + hex.EncodeToString(bytes), nil
}

// generateAPISecret generates a new API secret
func (ar *ApplicationRepository) generateAPISecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "as_" + hex.EncodeToString(bytes), nil
}

// matchesApplicationFilter checks if an application matches the given filter criteria
func (ar *ApplicationRepository) matchesApplicationFilter(app *portal.Application, filter *portal.ApplicationFilter) bool {
	// Filter by user ID
	if filter.UserID != "" && app.UserID != filter.UserID {
		return false
	}

	// Filter by name (partial match)
	if filter.Name != "" && !strings.Contains(strings.ToLower(app.Name), strings.ToLower(filter.Name)) {
		return false
	}

	// Filter by status
	if filter.Status != "" && app.Status != filter.Status {
		return false
	}

	// Filter by search (searches in name and description)
	if filter.Search != "" {
		searchLower := strings.ToLower(filter.Search)
		if !strings.Contains(strings.ToLower(app.Name), searchLower) &&
			!strings.Contains(strings.ToLower(app.Description), searchLower) {
			return false
		}
	}

	// Filter by creation date range
	if filter.CreatedAfter != nil && app.CreatedAt.Before(*filter.CreatedAfter) {
		return false
	}
	if filter.CreatedBefore != nil && app.CreatedAt.After(*filter.CreatedBefore) {
		return false
	}

	return true
}

// sortApplications sorts applications based on the given criteria
func (ar *ApplicationRepository) sortApplications(apps []*portal.Application, sortBy, sortOrder string) {
	sort.Slice(apps, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "id":
			less = apps[i].ID < apps[j].ID
		case "name":
			less = apps[i].Name < apps[j].Name
		case "user_id":
			less = apps[i].UserID < apps[j].UserID
		case "status":
			less = string(apps[i].Status) < string(apps[j].Status)
		case "rate_limit":
			less = apps[i].RateLimit < apps[j].RateLimit
		case "updated_at":
			less = apps[i].UpdatedAt.Before(apps[j].UpdatedAt)
		case "created_at":
		default:
			less = apps[i].CreatedAt.Before(apps[j].CreatedAt)
		}

		if sortOrder == "desc" {
			return !less
		}
		return less
	})
}
