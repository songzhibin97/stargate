package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/songzhibin97/stargate/pkg/portal"
)

// ApplicationRepository implements the portal.ApplicationRepository interface using PostgreSQL
type ApplicationRepository struct {
	repo *Repository
	tx   *Transaction
}

// NewApplicationRepository creates a new PostgreSQL application repository
func NewApplicationRepository(repo *Repository) *ApplicationRepository {
	return &ApplicationRepository{
		repo: repo,
	}
}

// CreateApplication creates a new application
func (ar *ApplicationRepository) CreateApplication(ctx context.Context, app *portal.Application) error {
	if err := ar.validateApplication(app); err != nil {
		return err
	}

	// Check if user exists
	userExists, err := ar.checkUserExists(ctx, app.UserID)
	if err != nil {
		return err
	}
	if !userExists {
		return portal.NewNotFoundError("USER_NOT_FOUND", "user not found")
	}

	query := `
		INSERT INTO applications (id, name, description, user_id, api_key, api_secret, status, rate_limit, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	now := time.Now()
	if app.CreatedAt.IsZero() {
		app.CreatedAt = now
	}
	app.UpdatedAt = now

	var execErr error
	if ar.tx != nil {
		_, execErr = ar.tx.execCommand(ctx, query, app.ID, app.Name, app.Description, app.UserID, app.APIKey, app.APISecret, app.Status, app.RateLimit, app.CreatedAt, app.UpdatedAt)
	} else {
		_, execErr = ar.repo.execCommand(ctx, query, app.ID, app.Name, app.Description, app.UserID, app.APIKey, app.APISecret, app.Status, app.RateLimit, app.CreatedAt, app.UpdatedAt)
	}

	if execErr != nil {
		if isUniqueViolation(execErr) {
			if strings.Contains(execErr.Error(), "applications_pkey") {
				return portal.NewConflictError("APPLICATION_ALREADY_EXISTS", "application with this ID already exists")
			}
			if strings.Contains(execErr.Error(), "applications_api_key_key") {
				return portal.NewConflictError("APPLICATION_API_KEY_EXISTS", "application with this API key already exists")
			}
		}
		if isForeignKeyViolation(execErr) {
			return portal.NewNotFoundError("USER_NOT_FOUND", "user not found")
		}
		return execErr
	}

	return nil
}

// GetApplication retrieves an application by ID
func (ar *ApplicationRepository) GetApplication(ctx context.Context, appID string) (*portal.Application, error) {
	if appID == "" {
		return nil, portal.NewValidationError("INVALID_APPLICATION_ID", "application ID cannot be empty")
	}

	query := `
		SELECT id, name, description, user_id, api_key, api_secret, status, rate_limit, created_at, updated_at
		FROM applications
		WHERE id = $1`

	var row *sql.Row
	if ar.tx != nil {
		row = ar.tx.execQueryRow(ctx, query, appID)
	} else {
		row = ar.repo.execQueryRow(ctx, query, appID)
	}

	app := &portal.Application{}
	err := row.Scan(&app.ID, &app.Name, &app.Description, &app.UserID, &app.APIKey, &app.APISecret, &app.Status, &app.RateLimit, &app.CreatedAt, &app.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, portal.NewNotFoundError("APPLICATION_NOT_FOUND", "application not found")
		}
		return nil, portal.NewDatabaseError("SCAN_FAILED", "failed to scan application", err)
	}

	return app, nil
}

// GetApplicationByAPIKey retrieves an application by API key
func (ar *ApplicationRepository) GetApplicationByAPIKey(ctx context.Context, apiKey string) (*portal.Application, error) {
	if apiKey == "" {
		return nil, portal.NewValidationError("INVALID_API_KEY", "API key cannot be empty")
	}

	query := `
		SELECT id, name, description, user_id, api_key, api_secret, status, rate_limit, created_at, updated_at
		FROM applications
		WHERE api_key = $1`

	var row *sql.Row
	if ar.tx != nil {
		row = ar.tx.execQueryRow(ctx, query, apiKey)
	} else {
		row = ar.repo.execQueryRow(ctx, query, apiKey)
	}

	app := &portal.Application{}
	err := row.Scan(&app.ID, &app.Name, &app.Description, &app.UserID, &app.APIKey, &app.APISecret, &app.Status, &app.RateLimit, &app.CreatedAt, &app.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, portal.NewNotFoundError("APPLICATION_NOT_FOUND", "application not found")
		}
		return nil, portal.NewDatabaseError("SCAN_FAILED", "failed to scan application", err)
	}

	return app, nil
}

// GetApplicationsByUser retrieves all applications for a specific user
func (ar *ApplicationRepository) GetApplicationsByUser(ctx context.Context, userID string) ([]*portal.Application, error) {
	if userID == "" {
		return nil, portal.NewValidationError("INVALID_USER_ID", "user ID cannot be empty")
	}

	query := `
		SELECT id, name, description, user_id, api_key, api_secret, status, rate_limit, created_at, updated_at
		FROM applications
		WHERE user_id = $1
		ORDER BY created_at DESC`

	var rows *sql.Rows
	var err error
	if ar.tx != nil {
		rows, err = ar.tx.execQuery(ctx, query, userID)
	} else {
		rows, err = ar.repo.execQuery(ctx, query, userID)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var applications []*portal.Application
	for rows.Next() {
		app := &portal.Application{}
		err := rows.Scan(&app.ID, &app.Name, &app.Description, &app.UserID, &app.APIKey, &app.APISecret, &app.Status, &app.RateLimit, &app.CreatedAt, &app.UpdatedAt)
		if err != nil {
			return nil, portal.NewDatabaseError("SCAN_FAILED", "failed to scan application", err)
		}
		applications = append(applications, app)
	}

	if err := rows.Err(); err != nil {
		return nil, portal.NewDatabaseError("ROWS_ERROR", "error iterating rows", err)
	}

	return applications, nil
}

// UpdateApplication updates an existing application
func (ar *ApplicationRepository) UpdateApplication(ctx context.Context, app *portal.Application) error {
	if err := ar.validateApplication(app); err != nil {
		return err
	}

	// Check if application exists
	existingApp, err := ar.GetApplication(ctx, app.ID)
	if err != nil {
		return err
	}

	// Check if user exists
	userExists, err := ar.checkUserExists(ctx, app.UserID)
	if err != nil {
		return err
	}
	if !userExists {
		return portal.NewNotFoundError("USER_NOT_FOUND", "user not found")
	}

	query := `
		UPDATE applications 
		SET name = $2, description = $3, user_id = $4, api_key = $5, api_secret = $6, status = $7, rate_limit = $8, updated_at = $9
		WHERE id = $1`

	app.CreatedAt = existingApp.CreatedAt // Preserve original creation time
	app.UpdatedAt = time.Now()

	var execErr error
	if ar.tx != nil {
		_, execErr = ar.tx.execCommand(ctx, query, app.ID, app.Name, app.Description, app.UserID, app.APIKey, app.APISecret, app.Status, app.RateLimit, app.UpdatedAt)
	} else {
		_, execErr = ar.repo.execCommand(ctx, query, app.ID, app.Name, app.Description, app.UserID, app.APIKey, app.APISecret, app.Status, app.RateLimit, app.UpdatedAt)
	}

	if execErr != nil {
		if isUniqueViolation(execErr) && strings.Contains(execErr.Error(), "applications_api_key_key") {
			return portal.NewConflictError("APPLICATION_API_KEY_EXISTS", "application with this API key already exists")
		}
		if isForeignKeyViolation(execErr) {
			return portal.NewNotFoundError("USER_NOT_FOUND", "user not found")
		}
		return execErr
	}

	return nil
}

// DeleteApplication deletes an application by ID
func (ar *ApplicationRepository) DeleteApplication(ctx context.Context, appID string) error {
	if appID == "" {
		return portal.NewValidationError("INVALID_APPLICATION_ID", "application ID cannot be empty")
	}

	// Check if application exists
	_, err := ar.GetApplication(ctx, appID)
	if err != nil {
		return err
	}

	query := `DELETE FROM applications WHERE id = $1`

	var result sql.Result
	if ar.tx != nil {
		result, err = ar.tx.execCommand(ctx, query, appID)
	} else {
		result, err = ar.repo.execCommand(ctx, query, appID)
	}

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return portal.NewDatabaseError("ROWS_AFFECTED_FAILED", "failed to get rows affected", err)
	}

	if rowsAffected == 0 {
		return portal.NewNotFoundError("APPLICATION_NOT_FOUND", "application not found")
	}

	return nil
}

// ExistsApplication checks if an application exists by ID
func (ar *ApplicationRepository) ExistsApplication(ctx context.Context, appID string) (bool, error) {
	if appID == "" {
		return false, portal.NewValidationError("INVALID_APPLICATION_ID", "application ID cannot be empty")
	}

	query := `SELECT 1 FROM applications WHERE id = $1 LIMIT 1`

	var exists int
	var row *sql.Row
	if ar.tx != nil {
		row = ar.tx.execQueryRow(ctx, query, appID)
	} else {
		row = ar.repo.execQueryRow(ctx, query, appID)
	}

	err := row.Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, portal.NewDatabaseError("SCAN_FAILED", "failed to check application existence", err)
	}

	return true, nil
}

// ExistsApplicationByAPIKey checks if an application exists by API key
func (ar *ApplicationRepository) ExistsApplicationByAPIKey(ctx context.Context, apiKey string) (bool, error) {
	if apiKey == "" {
		return false, portal.NewValidationError("INVALID_API_KEY", "API key cannot be empty")
	}

	query := `SELECT 1 FROM applications WHERE api_key = $1 LIMIT 1`

	var exists int
	var row *sql.Row
	if ar.tx != nil {
		row = ar.tx.execQueryRow(ctx, query, apiKey)
	} else {
		row = ar.repo.execQueryRow(ctx, query, apiKey)
	}

	err := row.Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, portal.NewDatabaseError("SCAN_FAILED", "failed to check application API key existence", err)
	}

	return true, nil
}

// UpdateApplicationStatus updates the status of an application
func (ar *ApplicationRepository) UpdateApplicationStatus(ctx context.Context, appID string, status portal.ApplicationStatus) error {
	if appID == "" {
		return portal.NewValidationError("INVALID_APPLICATION_ID", "application ID cannot be empty")
	}

	query := `UPDATE applications SET status = $2, updated_at = $3 WHERE id = $1`

	var result sql.Result
	var err error
	if ar.tx != nil {
		result, err = ar.tx.execCommand(ctx, query, appID, status, time.Now())
	} else {
		result, err = ar.repo.execCommand(ctx, query, appID, status, time.Now())
	}

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return portal.NewDatabaseError("ROWS_AFFECTED_FAILED", "failed to get rows affected", err)
	}

	if rowsAffected == 0 {
		return portal.NewNotFoundError("APPLICATION_NOT_FOUND", "application not found")
	}

	return nil
}

// UpdateApplicationRateLimit updates the rate limit of an application
func (ar *ApplicationRepository) UpdateApplicationRateLimit(ctx context.Context, appID string, rateLimit int64) error {
	if appID == "" {
		return portal.NewValidationError("INVALID_APPLICATION_ID", "application ID cannot be empty")
	}

	if rateLimit < 0 {
		return portal.NewValidationError("INVALID_RATE_LIMIT", "rate limit cannot be negative")
	}

	query := `UPDATE applications SET rate_limit = $2, updated_at = $3 WHERE id = $1`

	var result sql.Result
	var err error
	if ar.tx != nil {
		result, err = ar.tx.execCommand(ctx, query, appID, rateLimit, time.Now())
	} else {
		result, err = ar.repo.execCommand(ctx, query, appID, rateLimit, time.Now())
	}

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return portal.NewDatabaseError("ROWS_AFFECTED_FAILED", "failed to get rows affected", err)
	}

	if rowsAffected == 0 {
		return portal.NewNotFoundError("APPLICATION_NOT_FOUND", "application not found")
	}

	return nil
}

// RegenerateAPIKey generates a new API key for an application
func (ar *ApplicationRepository) RegenerateAPIKey(ctx context.Context, appID string) (string, error) {
	if appID == "" {
		return "", portal.NewValidationError("INVALID_APPLICATION_ID", "application ID cannot be empty")
	}

	// Check if application exists
	_, err := ar.GetApplication(ctx, appID)
	if err != nil {
		return "", err
	}

	// Generate new API key
	newAPIKey, err := ar.generateAPIKey()
	if err != nil {
		return "", portal.NewInternalError("API_KEY_GENERATION_FAILED", "failed to generate API key", err)
	}

	query := `UPDATE applications SET api_key = $2, updated_at = $3 WHERE id = $1`

	var result sql.Result
	if ar.tx != nil {
		result, err = ar.tx.execCommand(ctx, query, appID, newAPIKey, time.Now())
	} else {
		result, err = ar.repo.execCommand(ctx, query, appID, newAPIKey, time.Now())
	}

	if err != nil {
		if isUniqueViolation(err) && strings.Contains(err.Error(), "applications_api_key_key") {
			// Retry with a new key if collision occurs (very unlikely)
			return ar.RegenerateAPIKey(ctx, appID)
		}
		return "", err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return "", portal.NewDatabaseError("ROWS_AFFECTED_FAILED", "failed to get rows affected", err)
	}

	if rowsAffected == 0 {
		return "", portal.NewNotFoundError("APPLICATION_NOT_FOUND", "application not found")
	}

	return newAPIKey, nil
}

// RegenerateAPISecret generates a new API secret for an application
func (ar *ApplicationRepository) RegenerateAPISecret(ctx context.Context, appID string) (string, error) {
	if appID == "" {
		return "", portal.NewValidationError("INVALID_APPLICATION_ID", "application ID cannot be empty")
	}

	// Check if application exists
	_, err := ar.GetApplication(ctx, appID)
	if err != nil {
		return "", err
	}

	// Generate new API secret
	newAPISecret, err := ar.generateAPISecret()
	if err != nil {
		return "", portal.NewInternalError("API_SECRET_GENERATION_FAILED", "failed to generate API secret", err)
	}

	query := `UPDATE applications SET api_secret = $2, updated_at = $3 WHERE id = $1`

	var result sql.Result
	if ar.tx != nil {
		result, err = ar.tx.execCommand(ctx, query, appID, newAPISecret, time.Now())
	} else {
		result, err = ar.repo.execCommand(ctx, query, appID, newAPISecret, time.Now())
	}

	if err != nil {
		return "", err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return "", portal.NewDatabaseError("ROWS_AFFECTED_FAILED", "failed to get rows affected", err)
	}

	if rowsAffected == 0 {
		return "", portal.NewNotFoundError("APPLICATION_NOT_FOUND", "application not found")
	}

	return newAPISecret, nil
}

// CountApplicationsByUser returns the count of applications for a specific user
func (ar *ApplicationRepository) CountApplicationsByUser(ctx context.Context, userID string) (int64, error) {
	if userID == "" {
		return 0, portal.NewValidationError("INVALID_USER_ID", "user ID cannot be empty")
	}

	query := `SELECT COUNT(*) FROM applications WHERE user_id = $1`

	var count int64
	var row *sql.Row
	if ar.tx != nil {
		row = ar.tx.execQueryRow(ctx, query, userID)
	} else {
		row = ar.repo.execQueryRow(ctx, query, userID)
	}

	if err := row.Scan(&count); err != nil {
		return 0, portal.NewDatabaseError("COUNT_FAILED", "failed to count applications", err)
	}

	return count, nil
}

// ListApplications retrieves applications based on filter criteria
func (ar *ApplicationRepository) ListApplications(ctx context.Context, filter *portal.ApplicationFilter) (*portal.PaginatedApplications, error) {
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

	// Build WHERE clause
	whereClause, args := ar.buildWhereClause(filter)

	// Build ORDER BY clause
	orderBy := ar.buildOrderByClause(filter.SortBy, filter.SortOrder)

	// Count total records
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM applications %s", whereClause)
	var total int64
	var row *sql.Row
	if ar.tx != nil {
		row = ar.tx.execQueryRow(ctx, countQuery, args...)
	} else {
		row = ar.repo.execQueryRow(ctx, countQuery, args...)
	}

	if err := row.Scan(&total); err != nil {
		return nil, portal.NewDatabaseError("COUNT_FAILED", "failed to count applications", err)
	}

	// Query applications with pagination
	query := fmt.Sprintf(`
		SELECT id, name, description, user_id, api_key, api_secret, status, rate_limit, created_at, updated_at
		FROM applications %s %s
		LIMIT $%d OFFSET $%d`,
		whereClause, orderBy, len(args)+1, len(args)+2)

	args = append(args, filter.Limit, filter.Offset)

	var rows *sql.Rows
	var err error
	if ar.tx != nil {
		rows, err = ar.tx.execQuery(ctx, query, args...)
	} else {
		rows, err = ar.repo.execQuery(ctx, query, args...)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var applications []*portal.Application
	for rows.Next() {
		app := &portal.Application{}
		err := rows.Scan(&app.ID, &app.Name, &app.Description, &app.UserID, &app.APIKey, &app.APISecret, &app.Status, &app.RateLimit, &app.CreatedAt, &app.UpdatedAt)
		if err != nil {
			return nil, portal.NewDatabaseError("SCAN_FAILED", "failed to scan application", err)
		}
		applications = append(applications, app)
	}

	if err := rows.Err(); err != nil {
		return nil, portal.NewDatabaseError("ROWS_ERROR", "error iterating rows", err)
	}

	hasMore := int64(filter.Offset)+int64(len(applications)) < total

	return &portal.PaginatedApplications{
		Applications: applications,
		Total:        total,
		Offset:       filter.Offset,
		Limit:        filter.Limit,
		HasMore:      hasMore,
	}, nil
}

// CountApplications returns the total count of applications matching the filter
func (ar *ApplicationRepository) CountApplications(ctx context.Context, filter *portal.ApplicationFilter) (int64, error) {
	if filter == nil {
		filter = &portal.ApplicationFilter{}
	}

	whereClause, args := ar.buildWhereClause(filter)
	query := fmt.Sprintf("SELECT COUNT(*) FROM applications %s", whereClause)

	var count int64
	var row *sql.Row
	if ar.tx != nil {
		row = ar.tx.execQueryRow(ctx, query, args...)
	} else {
		row = ar.repo.execQueryRow(ctx, query, args...)
	}

	if err := row.Scan(&count); err != nil {
		return 0, portal.NewDatabaseError("COUNT_FAILED", "failed to count applications", err)
	}

	return count, nil
}

// BatchCreateApplications creates multiple applications in a single transaction
func (ar *ApplicationRepository) BatchCreateApplications(ctx context.Context, apps []*portal.Application) error {
	if len(apps) == 0 {
		return nil
	}

	// Validate all applications first
	for _, app := range apps {
		if err := ar.validateApplication(app); err != nil {
			return err
		}
	}

	// Use a transaction if not already in one
	if ar.tx == nil {
		tx, err := ar.repo.BeginTx(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)

		txAppRepo := tx.ApplicationRepository().(*ApplicationRepository)
		if err := txAppRepo.BatchCreateApplications(ctx, apps); err != nil {
			return err
		}

		return tx.Commit(ctx)
	}

	// Insert applications in batch
	query := `
		INSERT INTO applications (id, name, description, user_id, api_key, api_secret, status, rate_limit, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	now := time.Now()
	for _, app := range apps {
		// Check if user exists
		userExists, err := ar.checkUserExists(ctx, app.UserID)
		if err != nil {
			return err
		}
		if !userExists {
			return portal.NewNotFoundError("USER_NOT_FOUND", fmt.Sprintf("user with ID %s not found", app.UserID))
		}

		if app.CreatedAt.IsZero() {
			app.CreatedAt = now
		}
		app.UpdatedAt = now

		_, err = ar.tx.execCommand(ctx, query, app.ID, app.Name, app.Description, app.UserID, app.APIKey, app.APISecret, app.Status, app.RateLimit, app.CreatedAt, app.UpdatedAt)
		if err != nil {
			if isUniqueViolation(err) {
				if strings.Contains(err.Error(), "applications_pkey") {
					return portal.NewConflictError("APPLICATION_ALREADY_EXISTS", fmt.Sprintf("application with ID %s already exists", app.ID))
				}
				if strings.Contains(err.Error(), "applications_api_key_key") {
					return portal.NewConflictError("APPLICATION_API_KEY_EXISTS", fmt.Sprintf("application with API key %s already exists", app.APIKey))
				}
			}
			if isForeignKeyViolation(err) {
				return portal.NewNotFoundError("USER_NOT_FOUND", fmt.Sprintf("user with ID %s not found", app.UserID))
			}
			return err
		}
	}

	return nil
}

// BatchUpdateApplications updates multiple applications in a single transaction
func (ar *ApplicationRepository) BatchUpdateApplications(ctx context.Context, apps []*portal.Application) error {
	if len(apps) == 0 {
		return nil
	}

	// Validate all applications first
	for _, app := range apps {
		if err := ar.validateApplication(app); err != nil {
			return err
		}
	}

	// Use a transaction if not already in one
	if ar.tx == nil {
		tx, err := ar.repo.BeginTx(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)

		txAppRepo := tx.ApplicationRepository().(*ApplicationRepository)
		if err := txAppRepo.BatchUpdateApplications(ctx, apps); err != nil {
			return err
		}

		return tx.Commit(ctx)
	}

	// Update applications in batch
	query := `
		UPDATE applications
		SET name = $2, description = $3, user_id = $4, api_key = $5, api_secret = $6, status = $7, rate_limit = $8, updated_at = $9
		WHERE id = $1`

	now := time.Now()
	for _, app := range apps {
		// Check if application exists
		existingApp, err := ar.GetApplication(ctx, app.ID)
		if err != nil {
			return err
		}

		// Check if user exists
		userExists, err := ar.checkUserExists(ctx, app.UserID)
		if err != nil {
			return err
		}
		if !userExists {
			return portal.NewNotFoundError("USER_NOT_FOUND", fmt.Sprintf("user with ID %s not found", app.UserID))
		}

		app.CreatedAt = existingApp.CreatedAt // Preserve original creation time
		app.UpdatedAt = now

		_, err = ar.tx.execCommand(ctx, query, app.ID, app.Name, app.Description, app.UserID, app.APIKey, app.APISecret, app.Status, app.RateLimit, app.UpdatedAt)
		if err != nil {
			if isUniqueViolation(err) && strings.Contains(err.Error(), "applications_api_key_key") {
				return portal.NewConflictError("APPLICATION_API_KEY_EXISTS", fmt.Sprintf("application with API key %s already exists", app.APIKey))
			}
			if isForeignKeyViolation(err) {
				return portal.NewNotFoundError("USER_NOT_FOUND", fmt.Sprintf("user with ID %s not found", app.UserID))
			}
			return err
		}
	}

	return nil
}

// BatchDeleteApplications deletes multiple applications by IDs
func (ar *ApplicationRepository) BatchDeleteApplications(ctx context.Context, appIDs []string) error {
	if len(appIDs) == 0 {
		return nil
	}

	// Validate all application IDs first
	for _, appID := range appIDs {
		if appID == "" {
			return portal.NewValidationError("INVALID_APPLICATION_ID", "application ID cannot be empty")
		}
	}

	// Use a transaction if not already in one
	if ar.tx == nil {
		tx, err := ar.repo.BeginTx(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)

		txAppRepo := tx.ApplicationRepository().(*ApplicationRepository)
		if err := txAppRepo.BatchDeleteApplications(ctx, appIDs); err != nil {
			return err
		}

		return tx.Commit(ctx)
	}

	// Check if all applications exist first
	for _, appID := range appIDs {
		_, err := ar.GetApplication(ctx, appID)
		if err != nil {
			return err
		}
	}

	// Delete applications in batch using IN clause
	placeholders := make([]string, len(appIDs))
	args := make([]interface{}, len(appIDs))
	for i, appID := range appIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = appID
	}

	query := fmt.Sprintf("DELETE FROM applications WHERE id IN (%s)", strings.Join(placeholders, ","))

	result, err := ar.tx.execCommand(ctx, query, args...)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return portal.NewDatabaseError("ROWS_AFFECTED_FAILED", "failed to get rows affected", err)
	}

	if rowsAffected != int64(len(appIDs)) {
		return portal.NewDatabaseError("PARTIAL_DELETE", "not all applications were deleted", nil)
	}

	return nil
}

// validateApplication validates application data
func (ar *ApplicationRepository) validateApplication(app *portal.Application) error {
	if app == nil {
		return portal.NewValidationError("INVALID_APPLICATION", "application cannot be nil")
	}
	if app.ID == "" {
		return portal.NewValidationError("INVALID_APPLICATION_ID", "application ID cannot be empty")
	}
	if app.Name == "" {
		return portal.NewValidationError("INVALID_APPLICATION_NAME", "application name cannot be empty")
	}
	if app.UserID == "" {
		return portal.NewValidationError("INVALID_APPLICATION_USER_ID", "application user ID cannot be empty")
	}
	if app.APIKey == "" {
		return portal.NewValidationError("INVALID_APPLICATION_API_KEY", "application API key cannot be empty")
	}
	if app.Status == "" {
		return portal.NewValidationError("INVALID_APPLICATION_STATUS", "application status cannot be empty")
	}
	return nil
}

// checkUserExists checks if a user exists
func (ar *ApplicationRepository) checkUserExists(ctx context.Context, userID string) (bool, error) {
	query := `SELECT 1 FROM users WHERE id = $1 LIMIT 1`

	var exists int
	var row *sql.Row
	if ar.tx != nil {
		row = ar.tx.execQueryRow(ctx, query, userID)
	} else {
		row = ar.repo.execQueryRow(ctx, query, userID)
	}

	err := row.Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, portal.NewDatabaseError("SCAN_FAILED", "failed to check user existence", err)
	}

	return true, nil
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

// buildWhereClause builds the WHERE clause for application filtering
func (ar *ApplicationRepository) buildWhereClause(filter *portal.ApplicationFilter) (string, []interface{}) {
	var conditions []string
	var args []interface{}
	argIndex := 1

	if filter.UserID != "" {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", argIndex))
		args = append(args, filter.UserID)
		argIndex++
	}

	if filter.Name != "" {
		conditions = append(conditions, fmt.Sprintf("name ILIKE $%d", argIndex))
		args = append(args, "%"+filter.Name+"%")
		argIndex++
	}

	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, filter.Status)
		argIndex++
	}

	if filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(name ILIKE $%d OR description ILIKE $%d)", argIndex, argIndex))
		args = append(args, "%"+filter.Search+"%")
		argIndex++
	}

	if filter.CreatedAfter != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIndex))
		args = append(args, *filter.CreatedAfter)
		argIndex++
	}

	if filter.CreatedBefore != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIndex))
		args = append(args, *filter.CreatedBefore)
		argIndex++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	return whereClause, args
}

// buildOrderByClause builds the ORDER BY clause
func (ar *ApplicationRepository) buildOrderByClause(sortBy, sortOrder string) string {
	validSortFields := map[string]bool{
		"id":          true,
		"name":        true,
		"user_id":     true,
		"status":      true,
		"rate_limit":  true,
		"created_at":  true,
		"updated_at":  true,
	}

	if !validSortFields[sortBy] {
		sortBy = "created_at"
	}

	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}

	return fmt.Sprintf("ORDER BY %s %s", sortBy, strings.ToUpper(sortOrder))
}
