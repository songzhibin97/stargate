package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/songzhibin97/stargate/internal/config"
)

// AuthMiddleware provides authentication middleware for Admin API
type AuthMiddleware struct {
	config *config.Config
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(cfg *config.Config) *AuthMiddleware {
	return &AuthMiddleware{
		config: cfg,
	}
}

// Middleware returns the authentication middleware function
func (am *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip authentication if disabled
		if !am.config.AdminAPI.Auth.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Skip authentication for health endpoints
		if r.URL.Path == "/health" || r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		// Try API Key authentication first
		if len(am.config.AdminAPI.Auth.APIKey.Keys) > 0 {
			if am.authenticateAPIKey(r) {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Try JWT authentication
		if am.config.AdminAPI.Auth.JWT.Secret != "" {
			if am.authenticateJWT(r) {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Authentication failed
		writeErrorResponse(w, http.StatusUnauthorized, "Authentication required", nil)
	})
}

// authenticateAPIKey validates API key authentication
func (am *AuthMiddleware) authenticateAPIKey(r *http.Request) bool {
	// Get API key from header
	apiKey := r.Header.Get(am.config.AdminAPI.Auth.APIKey.Header)
	if apiKey == "" {
		return false
	}

	// Check if API key is valid
	for _, validKey := range am.config.AdminAPI.Auth.APIKey.Keys {
		if apiKey == validKey {
			return true
		}
	}

	return false
}

// authenticateJWT validates JWT token authentication
func (am *AuthMiddleware) authenticateJWT(r *http.Request) bool {
	// Get token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return false
	}

	// Extract token from "Bearer <token>" format
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return false
	}

	tokenString := parts[1]

	// Parse and validate token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Return the secret key
		return []byte(am.config.AdminAPI.Auth.JWT.Secret), nil
	})

	if err != nil {
		return false
	}

	// Check if token is valid
	if !token.Valid {
		return false
	}

	// Validate claims
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		// Check expiration
		if exp, ok := claims["exp"].(float64); ok {
			if time.Now().Unix() > int64(exp) {
				return false
			}
		}

		// Add claims to request context for later use
		ctx := context.WithValue(r.Context(), "jwt_claims", claims)
		*r = *r.WithContext(ctx)

		return true
	}

	return false
}

// JWTManager handles JWT token generation and validation
type JWTManager struct {
	config *config.Config
}

// NewJWTManager creates a new JWT manager
func NewJWTManager(cfg *config.Config) *JWTManager {
	return &JWTManager{
		config: cfg,
	}
}

// GenerateToken generates a new JWT token
func (jm *JWTManager) GenerateToken(userID string, permissions []string) (string, error) {
	// Get expiration duration
	expiresIn := jm.config.AdminAPI.Auth.JWT.ExpiresIn
	if expiresIn == 0 {
		expiresIn = 24 * time.Hour // default to 24 hours
	}

	// Create claims
	claims := jwt.MapClaims{
		"user_id":     userID,
		"permissions": permissions,
		"iat":         time.Now().Unix(),
		"exp":         time.Now().Add(expiresIn).Unix(),
		"iss":         "stargate-controller",
	}

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign token
	tokenString, err := token.SignedString([]byte(jm.config.AdminAPI.Auth.JWT.Secret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// ValidateToken validates a JWT token and returns claims
func (jm *JWTManager) ValidateToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(jm.config.AdminAPI.Auth.JWT.Secret), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims")
	}

	return claims, nil
}

// APIKeyManager handles API key generation and validation
type APIKeyManager struct {
	config *config.Config
}

// NewAPIKeyManager creates a new API key manager
func NewAPIKeyManager(cfg *config.Config) *APIKeyManager {
	return &APIKeyManager{
		config: cfg,
	}
}

// GenerateAPIKey generates a new API key
func (akm *APIKeyManager) GenerateAPIKey(prefix string) string {
	// Generate a random key using current timestamp and HMAC
	data := fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	h := hmac.New(sha256.New, []byte(akm.config.AdminAPI.Auth.JWT.Secret))
	h.Write([]byte(data))
	hash := hex.EncodeToString(h.Sum(nil))
	
	return fmt.Sprintf("%s_%s", prefix, hash[:32])
}

// ValidateAPIKey validates an API key
func (akm *APIKeyManager) ValidateAPIKey(apiKey string) bool {
	for _, validKey := range akm.config.AdminAPI.Auth.APIKey.Keys {
		if apiKey == validKey {
			return true
		}
	}
	return false
}

// AuthHandler handles authentication-related endpoints
type AuthHandler struct {
	config     *config.Config
	jwtManager *JWTManager
	apiManager *APIKeyManager
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		config:     cfg,
		jwtManager: NewJWTManager(cfg),
		apiManager: NewAPIKeyManager(cfg),
	}
}

// Login handles POST /auth/login
func (ah *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// For demo purposes, we'll accept any credentials and generate a token
	// In production, you would validate against a user database
	
	var loginReq struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := decodeJSONBody(r, &loginReq); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON", err)
		return
	}

	// Simple validation (replace with real authentication)
	if loginReq.Username == "" || loginReq.Password == "" {
		writeErrorResponse(w, http.StatusBadRequest, "Username and password required", nil)
		return
	}

	// Generate JWT token
	token, err := ah.jwtManager.GenerateToken(loginReq.Username, []string{"admin"})
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to generate token", err)
		return
	}

	response := map[string]interface{}{
		"token":      token,
		"token_type": "Bearer",
		"expires_in": ah.config.AdminAPI.Auth.JWT.ExpiresIn,
	}

	w.Header().Set("Content-Type", "application/json")
	writeJSONResponse(w, response)
}

// GenerateAPIKey handles POST /auth/api-keys
func (ah *AuthHandler) GenerateAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}

	if err := decodeJSONBody(r, &req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON", err)
		return
	}

	if req.Name == "" {
		req.Name = "api-key"
	}

	// Generate new API key
	apiKey := ah.apiManager.GenerateAPIKey(req.Name)

	response := map[string]interface{}{
		"api_key": apiKey,
		"name":    req.Name,
		"created": time.Now().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	writeJSONResponse(w, response)
}

// Helper functions
func decodeJSONBody(r *http.Request, dst interface{}) error {
	return json.NewDecoder(r.Body).Decode(dst)
}

func writeJSONResponse(w http.ResponseWriter, data interface{}) {
	json.NewEncoder(w).Encode(data)
}
