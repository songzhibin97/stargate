package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/songzhibin97/stargate/pkg/portal"
)

// UserRepository implements the portal.UserRepository interface using PostgreSQL
type UserRepository struct {
	repo *Repository
	tx   *Transaction
}

// NewUserRepository creates a new PostgreSQL user repository
func NewUserRepository(repo *Repository) *UserRepository {
	return &UserRepository{
		repo: repo,
	}
}

// CreateUser creates a new user
func (ur *UserRepository) CreateUser(ctx context.Context, user *portal.User) error {
	if err := ur.validateUser(user); err != nil {
		return err
	}

	query := `
		INSERT INTO users (id, email, name, password, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	now := time.Now()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	user.UpdatedAt = now

	var err error
	if ur.tx != nil {
		_, err = ur.tx.execCommand(ctx, query, user.ID, user.Email, user.Name, user.Password, user.Role, user.Status, user.CreatedAt, user.UpdatedAt)
	} else {
		_, err = ur.repo.execCommand(ctx, query, user.ID, user.Email, user.Name, user.Password, user.Role, user.Status, user.CreatedAt, user.UpdatedAt)
	}

	if err != nil {
		if isUniqueViolation(err) {
			if strings.Contains(err.Error(), "users_pkey") {
				return portal.NewConflictError("USER_ALREADY_EXISTS", "user with this ID already exists")
			}
			if strings.Contains(err.Error(), "users_email_key") {
				return portal.NewConflictError("USER_EMAIL_EXISTS", "user with this email already exists")
			}
		}
		return err
	}

	return nil
}

// GetUser retrieves a user by ID
func (ur *UserRepository) GetUser(ctx context.Context, userID string) (*portal.User, error) {
	if userID == "" {
		return nil, portal.NewValidationError("INVALID_USER_ID", "user ID cannot be empty")
	}

	query := `
		SELECT id, email, name, password, role, status, created_at, updated_at
		FROM users
		WHERE id = $1`

	var row *sql.Row
	if ur.tx != nil {
		row = ur.tx.execQueryRow(ctx, query, userID)
	} else {
		row = ur.repo.execQueryRow(ctx, query, userID)
	}

	user := &portal.User{}
	err := row.Scan(&user.ID, &user.Email, &user.Name, &user.Password, &user.Role, &user.Status, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, portal.NewNotFoundError("USER_NOT_FOUND", "user not found")
		}
		return nil, portal.NewDatabaseError("SCAN_FAILED", "failed to scan user", err)
	}

	return user, nil
}

// GetUserByEmail retrieves a user by email address
func (ur *UserRepository) GetUserByEmail(ctx context.Context, email string) (*portal.User, error) {
	if email == "" {
		return nil, portal.NewValidationError("INVALID_EMAIL", "email cannot be empty")
	}

	query := `
		SELECT id, email, name, password, role, status, created_at, updated_at
		FROM users
		WHERE email = $1`

	var row *sql.Row
	if ur.tx != nil {
		row = ur.tx.execQueryRow(ctx, query, email)
	} else {
		row = ur.repo.execQueryRow(ctx, query, email)
	}

	user := &portal.User{}
	err := row.Scan(&user.ID, &user.Email, &user.Name, &user.Password, &user.Role, &user.Status, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, portal.NewNotFoundError("USER_NOT_FOUND", "user not found")
		}
		return nil, portal.NewDatabaseError("SCAN_FAILED", "failed to scan user", err)
	}

	return user, nil
}

// UpdateUser updates an existing user
func (ur *UserRepository) UpdateUser(ctx context.Context, user *portal.User) error {
	if err := ur.validateUser(user); err != nil {
		return err
	}

	// Check if user exists
	existingUser, err := ur.GetUser(ctx, user.ID)
	if err != nil {
		return err
	}

	query := `
		UPDATE users 
		SET email = $2, name = $3, role = $4, status = $5, updated_at = $6
		WHERE id = $1`

	user.CreatedAt = existingUser.CreatedAt // Preserve original creation time
	user.UpdatedAt = time.Now()

	if ur.tx != nil {
		_, err = ur.tx.execCommand(ctx, query, user.ID, user.Email, user.Name, user.Role, user.Status, user.UpdatedAt)
	} else {
		_, err = ur.repo.execCommand(ctx, query, user.ID, user.Email, user.Name, user.Role, user.Status, user.UpdatedAt)
	}

	if err != nil {
		if isUniqueViolation(err) && strings.Contains(err.Error(), "users_email_key") {
			return portal.NewConflictError("USER_EMAIL_EXISTS", "user with this email already exists")
		}
		return err
	}

	return nil
}

// DeleteUser deletes a user by ID
func (ur *UserRepository) DeleteUser(ctx context.Context, userID string) error {
	if userID == "" {
		return portal.NewValidationError("INVALID_USER_ID", "user ID cannot be empty")
	}

	// Check if user exists
	_, err := ur.GetUser(ctx, userID)
	if err != nil {
		return err
	}

	query := `DELETE FROM users WHERE id = $1`

	var result sql.Result
	if ur.tx != nil {
		result, err = ur.tx.execCommand(ctx, query, userID)
	} else {
		result, err = ur.repo.execCommand(ctx, query, userID)
	}

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return portal.NewDatabaseError("ROWS_AFFECTED_FAILED", "failed to get rows affected", err)
	}

	if rowsAffected == 0 {
		return portal.NewNotFoundError("USER_NOT_FOUND", "user not found")
	}

	return nil
}

// ExistsUser checks if a user exists by ID
func (ur *UserRepository) ExistsUser(ctx context.Context, userID string) (bool, error) {
	if userID == "" {
		return false, portal.NewValidationError("INVALID_USER_ID", "user ID cannot be empty")
	}

	query := `SELECT 1 FROM users WHERE id = $1 LIMIT 1`

	var exists int
	var row *sql.Row
	if ur.tx != nil {
		row = ur.tx.execQueryRow(ctx, query, userID)
	} else {
		row = ur.repo.execQueryRow(ctx, query, userID)
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

// ExistsUserByEmail checks if a user exists by email
func (ur *UserRepository) ExistsUserByEmail(ctx context.Context, email string) (bool, error) {
	if email == "" {
		return false, portal.NewValidationError("INVALID_EMAIL", "email cannot be empty")
	}

	query := `SELECT 1 FROM users WHERE email = $1 LIMIT 1`

	var exists int
	var row *sql.Row
	if ur.tx != nil {
		row = ur.tx.execQueryRow(ctx, query, email)
	} else {
		row = ur.repo.execQueryRow(ctx, query, email)
	}

	err := row.Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, portal.NewDatabaseError("SCAN_FAILED", "failed to check user email existence", err)
	}

	return true, nil
}

// UpdateUserStatus updates the status of a user
func (ur *UserRepository) UpdateUserStatus(ctx context.Context, userID string, status portal.UserStatus) error {
	if userID == "" {
		return portal.NewValidationError("INVALID_USER_ID", "user ID cannot be empty")
	}

	query := `UPDATE users SET status = $2, updated_at = $3 WHERE id = $1`

	var result sql.Result
	var err error
	if ur.tx != nil {
		result, err = ur.tx.execCommand(ctx, query, userID, status, time.Now())
	} else {
		result, err = ur.repo.execCommand(ctx, query, userID, status, time.Now())
	}

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return portal.NewDatabaseError("ROWS_AFFECTED_FAILED", "failed to get rows affected", err)
	}

	if rowsAffected == 0 {
		return portal.NewNotFoundError("USER_NOT_FOUND", "user not found")
	}

	return nil
}

// UpdateUserRole updates the role of a user
func (ur *UserRepository) UpdateUserRole(ctx context.Context, userID string, role portal.UserRole) error {
	if userID == "" {
		return portal.NewValidationError("INVALID_USER_ID", "user ID cannot be empty")
	}

	query := `UPDATE users SET role = $2, updated_at = $3 WHERE id = $1`

	var result sql.Result
	var err error
	if ur.tx != nil {
		result, err = ur.tx.execCommand(ctx, query, userID, role, time.Now())
	} else {
		result, err = ur.repo.execCommand(ctx, query, userID, role, time.Now())
	}

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return portal.NewDatabaseError("ROWS_AFFECTED_FAILED", "failed to get rows affected", err)
	}

	if rowsAffected == 0 {
		return portal.NewNotFoundError("USER_NOT_FOUND", "user not found")
	}

	return nil
}

// validateUser validates user data
func (ur *UserRepository) validateUser(user *portal.User) error {
	if user == nil {
		return portal.NewValidationError("INVALID_USER", "user cannot be nil")
	}
	if user.ID == "" {
		return portal.NewValidationError("INVALID_USER_ID", "user ID cannot be empty")
	}
	if user.Email == "" {
		return portal.NewValidationError("INVALID_USER_EMAIL", "user email cannot be empty")
	}
	if user.Name == "" {
		return portal.NewValidationError("INVALID_USER_NAME", "user name cannot be empty")
	}
	if user.Role == "" {
		return portal.NewValidationError("INVALID_USER_ROLE", "user role cannot be empty")
	}
	if user.Status == "" {
		return portal.NewValidationError("INVALID_USER_STATUS", "user status cannot be empty")
	}
	return nil
}

// ListUsers retrieves users based on filter criteria
func (ur *UserRepository) ListUsers(ctx context.Context, filter *portal.UserFilter) (*portal.PaginatedUsers, error) {
	if filter == nil {
		filter = &portal.UserFilter{}
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
	whereClause, args := ur.buildWhereClause(filter)

	// Build ORDER BY clause
	orderBy := ur.buildOrderByClause(filter.SortBy, filter.SortOrder)

	// Count total records
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM users %s", whereClause)
	var total int64
	var row *sql.Row
	if ur.tx != nil {
		row = ur.tx.execQueryRow(ctx, countQuery, args...)
	} else {
		row = ur.repo.execQueryRow(ctx, countQuery, args...)
	}

	if err := row.Scan(&total); err != nil {
		return nil, portal.NewDatabaseError("COUNT_FAILED", "failed to count users", err)
	}

	// Query users with pagination
	query := fmt.Sprintf(`
		SELECT id, email, name, role, status, created_at, updated_at
		FROM users %s %s
		LIMIT $%d OFFSET $%d`,
		whereClause, orderBy, len(args)+1, len(args)+2)

	args = append(args, filter.Limit, filter.Offset)

	var rows *sql.Rows
	var err error
	if ur.tx != nil {
		rows, err = ur.tx.execQuery(ctx, query, args...)
	} else {
		rows, err = ur.repo.execQuery(ctx, query, args...)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*portal.User
	for rows.Next() {
		user := &portal.User{}
		err := rows.Scan(&user.ID, &user.Email, &user.Name, &user.Role, &user.Status, &user.CreatedAt, &user.UpdatedAt)
		if err != nil {
			return nil, portal.NewDatabaseError("SCAN_FAILED", "failed to scan user", err)
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, portal.NewDatabaseError("ROWS_ERROR", "error iterating rows", err)
	}

	hasMore := int64(filter.Offset)+int64(len(users)) < total

	return &portal.PaginatedUsers{
		Users:   users,
		Total:   total,
		Offset:  filter.Offset,
		Limit:   filter.Limit,
		HasMore: hasMore,
	}, nil
}

// CountUsers returns the total count of users matching the filter
func (ur *UserRepository) CountUsers(ctx context.Context, filter *portal.UserFilter) (int64, error) {
	if filter == nil {
		filter = &portal.UserFilter{}
	}

	whereClause, args := ur.buildWhereClause(filter)
	query := fmt.Sprintf("SELECT COUNT(*) FROM users %s", whereClause)

	var count int64
	var row *sql.Row
	if ur.tx != nil {
		row = ur.tx.execQueryRow(ctx, query, args...)
	} else {
		row = ur.repo.execQueryRow(ctx, query, args...)
	}

	if err := row.Scan(&count); err != nil {
		return 0, portal.NewDatabaseError("COUNT_FAILED", "failed to count users", err)
	}

	return count, nil
}

// BatchCreateUsers creates multiple users in a single transaction
func (ur *UserRepository) BatchCreateUsers(ctx context.Context, users []*portal.User) error {
	if len(users) == 0 {
		return nil
	}

	// Validate all users first
	for _, user := range users {
		if err := ur.validateUser(user); err != nil {
			return err
		}
	}

	// Use a transaction if not already in one
	if ur.tx == nil {
		tx, err := ur.repo.BeginTx(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)

		txUserRepo := tx.UserRepository().(*UserRepository)
		if err := txUserRepo.BatchCreateUsers(ctx, users); err != nil {
			return err
		}

		return tx.Commit(ctx)
	}

	// Insert users in batch
	query := `
		INSERT INTO users (id, email, name, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	now := time.Now()
	for _, user := range users {
		if user.CreatedAt.IsZero() {
			user.CreatedAt = now
		}
		user.UpdatedAt = now

		_, err := ur.tx.execCommand(ctx, query, user.ID, user.Email, user.Name, user.Role, user.Status, user.CreatedAt, user.UpdatedAt)
		if err != nil {
			if isUniqueViolation(err) {
				if strings.Contains(err.Error(), "users_pkey") {
					return portal.NewConflictError("USER_ALREADY_EXISTS", fmt.Sprintf("user with ID %s already exists", user.ID))
				}
				if strings.Contains(err.Error(), "users_email_key") {
					return portal.NewConflictError("USER_EMAIL_EXISTS", fmt.Sprintf("user with email %s already exists", user.Email))
				}
			}
			return err
		}
	}

	return nil
}

// BatchUpdateUsers updates multiple users in a single transaction
func (ur *UserRepository) BatchUpdateUsers(ctx context.Context, users []*portal.User) error {
	if len(users) == 0 {
		return nil
	}

	// Validate all users first
	for _, user := range users {
		if err := ur.validateUser(user); err != nil {
			return err
		}
	}

	// Use a transaction if not already in one
	if ur.tx == nil {
		tx, err := ur.repo.BeginTx(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)

		txUserRepo := tx.UserRepository().(*UserRepository)
		if err := txUserRepo.BatchUpdateUsers(ctx, users); err != nil {
			return err
		}

		return tx.Commit(ctx)
	}

	// Update users in batch
	query := `
		UPDATE users
		SET email = $2, name = $3, role = $4, status = $5, updated_at = $6
		WHERE id = $1`

	now := time.Now()
	for _, user := range users {
		// Check if user exists
		existingUser, err := ur.GetUser(ctx, user.ID)
		if err != nil {
			return err
		}

		user.CreatedAt = existingUser.CreatedAt // Preserve original creation time
		user.UpdatedAt = now

		_, err = ur.tx.execCommand(ctx, query, user.ID, user.Email, user.Name, user.Role, user.Status, user.UpdatedAt)
		if err != nil {
			if isUniqueViolation(err) && strings.Contains(err.Error(), "users_email_key") {
				return portal.NewConflictError("USER_EMAIL_EXISTS", fmt.Sprintf("user with email %s already exists", user.Email))
			}
			return err
		}
	}

	return nil
}

// BatchDeleteUsers deletes multiple users by IDs
func (ur *UserRepository) BatchDeleteUsers(ctx context.Context, userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}

	// Validate all user IDs first
	for _, userID := range userIDs {
		if userID == "" {
			return portal.NewValidationError("INVALID_USER_ID", "user ID cannot be empty")
		}
	}

	// Use a transaction if not already in one
	if ur.tx == nil {
		tx, err := ur.repo.BeginTx(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)

		txUserRepo := tx.UserRepository().(*UserRepository)
		if err := txUserRepo.BatchDeleteUsers(ctx, userIDs); err != nil {
			return err
		}

		return tx.Commit(ctx)
	}

	// Check if all users exist first
	for _, userID := range userIDs {
		_, err := ur.GetUser(ctx, userID)
		if err != nil {
			return err
		}
	}

	// Delete users in batch using IN clause
	placeholders := make([]string, len(userIDs))
	args := make([]interface{}, len(userIDs))
	for i, userID := range userIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = userID
	}

	query := fmt.Sprintf("DELETE FROM users WHERE id IN (%s)", strings.Join(placeholders, ","))

	result, err := ur.tx.execCommand(ctx, query, args...)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return portal.NewDatabaseError("ROWS_AFFECTED_FAILED", "failed to get rows affected", err)
	}

	if rowsAffected != int64(len(userIDs)) {
		return portal.NewDatabaseError("PARTIAL_DELETE", "not all users were deleted", nil)
	}

	return nil
}

// buildWhereClause builds the WHERE clause for user filtering
func (ur *UserRepository) buildWhereClause(filter *portal.UserFilter) (string, []interface{}) {
	var conditions []string
	var args []interface{}
	argIndex := 1

	if filter.Email != "" {
		conditions = append(conditions, fmt.Sprintf("email = $%d", argIndex))
		args = append(args, filter.Email)
		argIndex++
	}

	if filter.Name != "" {
		conditions = append(conditions, fmt.Sprintf("name ILIKE $%d", argIndex))
		args = append(args, "%"+filter.Name+"%")
		argIndex++
	}

	if filter.Role != "" {
		conditions = append(conditions, fmt.Sprintf("role = $%d", argIndex))
		args = append(args, filter.Role)
		argIndex++
	}

	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, filter.Status)
		argIndex++
	}

	if filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(name ILIKE $%d OR email ILIKE $%d)", argIndex, argIndex))
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
func (ur *UserRepository) buildOrderByClause(sortBy, sortOrder string) string {
	validSortFields := map[string]bool{
		"id":         true,
		"email":      true,
		"name":       true,
		"role":       true,
		"status":     true,
		"created_at": true,
		"updated_at": true,
	}

	if !validSortFields[sortBy] {
		sortBy = "created_at"
	}

	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}

	return fmt.Sprintf("ORDER BY %s %s", sortBy, strings.ToUpper(sortOrder))
}
