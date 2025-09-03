package portal

import (
	"errors"
	"fmt"
)

// Common error variables
var (
	// User errors
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrInvalidUserEmail  = errors.New("invalid user email")
	ErrInvalidUserRole   = errors.New("invalid user role")
	ErrInvalidUserStatus = errors.New("invalid user status")
	
	// Application errors
	ErrApplicationNotFound      = errors.New("application not found")
	ErrApplicationAlreadyExists = errors.New("application already exists")
	ErrInvalidApplicationName   = errors.New("invalid application name")
	ErrInvalidApplicationStatus = errors.New("invalid application status")
	ErrApplicationLimitExceeded = errors.New("application limit exceeded")
	
	// General errors
	ErrInvalidInput      = errors.New("invalid input")
	ErrValidationFailed  = errors.New("validation failed")
	ErrDatabaseError     = errors.New("database error")
	ErrTransactionFailed = errors.New("transaction failed")
	ErrPermissionDenied  = errors.New("permission denied")
	ErrInternalError     = errors.New("internal error")
)

// ErrorType represents the type of error
type ErrorType string

const (
	ErrorTypeNotFound     ErrorType = "not_found"
	ErrorTypeConflict     ErrorType = "conflict"
	ErrorTypeValidation   ErrorType = "validation"
	ErrorTypePermission   ErrorType = "permission"
	ErrorTypeDatabase     ErrorType = "database"
	ErrorTypeInternal     ErrorType = "internal"
)

// PortalError represents a structured error with additional context
type PortalError struct {
	Type    ErrorType `json:"type"`
	Code    string    `json:"code"`
	Message string    `json:"message"`
	Details string    `json:"details,omitempty"`
	Cause   error     `json:"-"`
}

// Error implements the error interface
func (e *PortalError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause
func (e *PortalError) Unwrap() error {
	return e.Cause
}

// NewNotFoundError creates a new not found error
func NewNotFoundError(code, message string) *PortalError {
	return &PortalError{
		Type:    ErrorTypeNotFound,
		Code:    code,
		Message: message,
	}
}

// NewConflictError creates a new conflict error
func NewConflictError(code, message string) *PortalError {
	return &PortalError{
		Type:    ErrorTypeConflict,
		Code:    code,
		Message: message,
	}
}

// NewValidationError creates a new validation error
func NewValidationError(code, message string) *PortalError {
	return &PortalError{
		Type:    ErrorTypeValidation,
		Code:    code,
		Message: message,
	}
}

// NewPermissionError creates a new permission error
func NewPermissionError(code, message string) *PortalError {
	return &PortalError{
		Type:    ErrorTypePermission,
		Code:    code,
		Message: message,
	}
}

// NewDatabaseError creates a new database error
func NewDatabaseError(code, message string, cause error) *PortalError {
	return &PortalError{
		Type:    ErrorTypeDatabase,
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// NewInternalError creates a new internal error
func NewInternalError(code, message string, cause error) *PortalError {
	return &PortalError{
		Type:    ErrorTypeInternal,
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// IsNotFoundError checks if the error is a not found error
func IsNotFoundError(err error) bool {
	var portalErr *PortalError
	if errors.As(err, &portalErr) {
		return portalErr.Type == ErrorTypeNotFound
	}
	return errors.Is(err, ErrUserNotFound) || errors.Is(err, ErrApplicationNotFound)
}

// IsConflictError checks if the error is a conflict error
func IsConflictError(err error) bool {
	var portalErr *PortalError
	if errors.As(err, &portalErr) {
		return portalErr.Type == ErrorTypeConflict
	}
	return errors.Is(err, ErrUserAlreadyExists) || errors.Is(err, ErrApplicationAlreadyExists)
}

// IsValidationError checks if the error is a validation error
func IsValidationError(err error) bool {
	var portalErr *PortalError
	if errors.As(err, &portalErr) {
		return portalErr.Type == ErrorTypeValidation
	}
	return errors.Is(err, ErrInvalidInput) || errors.Is(err, ErrValidationFailed)
}

// IsPermissionError checks if the error is a permission error
func IsPermissionError(err error) bool {
	var portalErr *PortalError
	if errors.As(err, &portalErr) {
		return portalErr.Type == ErrorTypePermission
	}
	return errors.Is(err, ErrPermissionDenied)
}

// IsDatabaseError checks if the error is a database error
func IsDatabaseError(err error) bool {
	var portalErr *PortalError
	if errors.As(err, &portalErr) {
		return portalErr.Type == ErrorTypeDatabase
	}
	return errors.Is(err, ErrDatabaseError) || errors.Is(err, ErrTransactionFailed)
}

// IsInternalError checks if the error is an internal error
func IsInternalError(err error) bool {
	var portalErr *PortalError
	if errors.As(err, &portalErr) {
		return portalErr.Type == ErrorTypeInternal
	}
	return errors.Is(err, ErrInternalError)
}
