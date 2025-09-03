package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/portal/auth"
	"github.com/songzhibin97/stargate/pkg/portal"
)

// PortalHandler handles portal API requests
type PortalHandler struct {
	config           *config.Config
	userRepo         portal.UserRepository
	passwordHasher   *auth.PasswordHasher
	jwtManager       *auth.JWTManager
	userIDGenerator  *auth.UserIDGenerator
}

// NewPortalHandler creates a new portal handler
func NewPortalHandler(cfg *config.Config, userRepo portal.UserRepository) (*PortalHandler, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if userRepo == nil {
		return nil, fmt.Errorf("user repository cannot be nil")
	}

	// Create JWT manager
	jwtManager, err := auth.NewJWTManager(
		cfg.Portal.JWT.Secret,
		cfg.Portal.JWT.Algorithm,
		cfg.Portal.JWT.ExpiresIn,
		cfg.Portal.JWT.Issuer,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create JWT manager: %w", err)
	}

	return &PortalHandler{
		config:          cfg,
		userRepo:        userRepo,
		passwordHasher:  auth.NewPasswordHasher(),
		jwtManager:      jwtManager,
		userIDGenerator: auth.NewUserIDGenerator(),
	}, nil
}

// RegisterRequest represents a user registration request
type RegisterRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

// LoginRequest represents a user login request
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResponse represents an authentication response
type AuthResponse struct {
	Token     string    `json:"token"`
	User      UserInfo  `json:"user"`
	ExpiresAt time.Time `json:"expires_at"`
}

// UserInfo represents user information in responses
type UserInfo struct {
	ID     string `json:"id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	Status string `json:"status"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// HandleRegister handles user registration
func (ph *PortalHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ph.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ph.writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON format")
		return
	}

	// Validate request
	if err := ph.validateRegisterRequest(&req); err != nil {
		ph.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	ctx := r.Context()

	// Check if user already exists
	existingUser, err := ph.userRepo.GetUserByEmail(ctx, req.Email)
	if err == nil && existingUser != nil {
		ph.writeError(w, http.StatusConflict, "USER_EXISTS", "User with this email already exists")
		return
	}

	// Hash password
	hashedPassword, err := ph.passwordHasher.HashPassword(req.Password)
	if err != nil {
		ph.writeError(w, http.StatusInternalServerError, "HASH_ERROR", "Failed to process password")
		return
	}

	// Generate user ID
	userID, err := ph.userIDGenerator.GenerateUserID()
	if err != nil {
		ph.writeError(w, http.StatusInternalServerError, "ID_GENERATION_ERROR", "Failed to generate user ID")
		return
	}

	// Create user
	user := &portal.User{
		ID:       userID,
		Email:    req.Email,
		Name:     req.Name,
		Role:     portal.UserRoleDeveloper, // Default role
		Status:   portal.UserStatusActive,  // Default status
		Password: hashedPassword,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := ph.userRepo.CreateUser(ctx, user); err != nil {
		if portal.IsConflictError(err) {
			ph.writeError(w, http.StatusConflict, "USER_EXISTS", "User with this email already exists")
		} else {
			ph.writeError(w, http.StatusInternalServerError, "CREATE_ERROR", "Failed to create user")
		}
		return
	}

	// Generate JWT token
	token, err := ph.jwtManager.GenerateToken(user.ID, user.Email, string(user.Role))
	if err != nil {
		ph.writeError(w, http.StatusInternalServerError, "TOKEN_ERROR", "Failed to generate token")
		return
	}

	// Prepare response
	response := AuthResponse{
		Token: token,
		User: UserInfo{
			ID:     user.ID,
			Email:  user.Email,
			Name:   user.Name,
			Role:   string(user.Role),
			Status: string(user.Status),
		},
		ExpiresAt: time.Now().Add(ph.config.Portal.JWT.ExpiresIn),
	}

	ph.writeJSON(w, http.StatusCreated, response)
}

// HandleLogin handles user login
func (ph *PortalHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ph.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ph.writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON format")
		return
	}

	// Validate request
	if err := ph.validateLoginRequest(&req); err != nil {
		ph.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	ctx := r.Context()

	// Get user by email
	user, err := ph.userRepo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		if portal.IsNotFoundError(err) {
			ph.writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password")
		} else {
			ph.writeError(w, http.StatusInternalServerError, "DATABASE_ERROR", "Failed to retrieve user")
		}
		return
	}

	// Check user status
	if user.Status != portal.UserStatusActive {
		ph.writeError(w, http.StatusUnauthorized, "USER_INACTIVE", "User account is not active")
		return
	}

	// Verify password
	if err := ph.passwordHasher.VerifyPassword(req.Password, user.Password); err != nil {
		ph.writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password")
		return
	}

	// Generate JWT token
	token, err := ph.jwtManager.GenerateToken(user.ID, user.Email, string(user.Role))
	if err != nil {
		ph.writeError(w, http.StatusInternalServerError, "TOKEN_ERROR", "Failed to generate token")
		return
	}

	// Prepare response
	response := AuthResponse{
		Token: token,
		User: UserInfo{
			ID:     user.ID,
			Email:  user.Email,
			Name:   user.Name,
			Role:   string(user.Role),
			Status: string(user.Status),
		},
		ExpiresAt: time.Now().Add(ph.config.Portal.JWT.ExpiresIn),
	}

	ph.writeJSON(w, http.StatusOK, response)
}

// validateRegisterRequest validates a registration request
func (ph *PortalHandler) validateRegisterRequest(req *RegisterRequest) error {
	if req.Email == "" {
		return fmt.Errorf("email is required")
	}
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	if req.Password == "" {
		return fmt.Errorf("password is required")
	}
	if len(req.Password) < 8 {
		return fmt.Errorf("password must be at least 8 characters long")
	}
	if !strings.Contains(req.Email, "@") {
		return fmt.Errorf("invalid email format")
	}
	return nil
}

// validateLoginRequest validates a login request
func (ph *PortalHandler) validateLoginRequest(req *LoginRequest) error {
	if req.Email == "" {
		return fmt.Errorf("email is required")
	}
	if req.Password == "" {
		return fmt.Errorf("password is required")
	}
	if !strings.Contains(req.Email, "@") {
		return fmt.Errorf("invalid email format")
	}
	return nil
}

// writeJSON writes a JSON response
func (ph *PortalHandler) writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// writeError writes an error response
func (ph *PortalHandler) writeError(w http.ResponseWriter, statusCode int, code, message string) {
	response := ErrorResponse{
		Error:   http.StatusText(statusCode),
		Message: message,
		Code:    code,
	}
	ph.writeJSON(w, statusCode, response)
}
