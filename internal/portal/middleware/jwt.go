package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/portal/auth"
)

// JWTMiddleware handles JWT authentication for Portal API endpoints
type JWTMiddleware struct {
	jwtManager *auth.JWTManager
	config     *config.Config
}

// NewJWTMiddleware creates a new JWT middleware
func NewJWTMiddleware(cfg *config.Config) (*JWTMiddleware, error) {
	jwtManager, err := auth.NewJWTManager(
		cfg.Portal.JWT.Secret,
		cfg.Portal.JWT.Algorithm,
		cfg.Portal.JWT.ExpiresIn,
		cfg.Portal.JWT.Issuer,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create JWT manager: %w", err)
	}

	return &JWTMiddleware{
		jwtManager: jwtManager,
		config:     cfg,
	}, nil
}

// UserContextKey is the key used to store user information in request context
type UserContextKey string

const (
	// UserIDKey is the context key for user ID
	UserIDKey UserContextKey = "user_id"
	// UserEmailKey is the context key for user email
	UserEmailKey UserContextKey = "user_email"
	// UserRoleKey is the context key for user role
	UserRoleKey UserContextKey = "user_role"
	// JWTClaimsKey is the context key for JWT claims
	JWTClaimsKey UserContextKey = "jwt_claims"
)

// RequireAuth is a middleware that requires valid JWT authentication
func (jm *JWTMiddleware) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract JWT token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			jm.writeError(w, http.StatusUnauthorized, "MISSING_TOKEN", "Authorization header is required")
			return
		}

		// Check if it's a Bearer token
		if !strings.HasPrefix(authHeader, "Bearer ") {
			jm.writeError(w, http.StatusUnauthorized, "INVALID_TOKEN_FORMAT", "Authorization header must be in format 'Bearer <token>'")
			return
		}

		// Extract token
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			jm.writeError(w, http.StatusUnauthorized, "EMPTY_TOKEN", "JWT token cannot be empty")
			return
		}

		// Validate token
		claims, err := jm.jwtManager.ValidateToken(token)
		if err != nil {
			jm.writeError(w, http.StatusUnauthorized, "INVALID_TOKEN", "Invalid or expired JWT token")
			return
		}

		// Add user information to request context
		ctx := r.Context()
		ctx = context.WithValue(ctx, UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, UserEmailKey, claims.Email)
		ctx = context.WithValue(ctx, UserRoleKey, claims.Role)
		ctx = context.WithValue(ctx, JWTClaimsKey, claims)

		// Call next handler with updated context
		next(w, r.WithContext(ctx))
	}
}

// RequireRole is a middleware that requires a specific user role
func (jm *JWTMiddleware) RequireRole(role string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return jm.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
			userRole := GetUserRole(r.Context())
			if userRole == "" {
				jm.writeError(w, http.StatusForbidden, "MISSING_ROLE", "User role not found in token")
				return
			}

			if userRole != role {
				jm.writeError(w, http.StatusForbidden, "INSUFFICIENT_PERMISSIONS", fmt.Sprintf("Required role: %s, user role: %s", role, userRole))
				return
			}

			next(w, r)
		})
	}
}

// RequireAnyRole is a middleware that requires any of the specified roles
func (jm *JWTMiddleware) RequireAnyRole(roles ...string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return jm.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
			userRole := GetUserRole(r.Context())
			if userRole == "" {
				jm.writeError(w, http.StatusForbidden, "MISSING_ROLE", "User role not found in token")
				return
			}

			// Check if user has any of the required roles
			hasRole := false
			for _, role := range roles {
				if userRole == role {
					hasRole = true
					break
				}
			}

			if !hasRole {
				jm.writeError(w, http.StatusForbidden, "INSUFFICIENT_PERMISSIONS", fmt.Sprintf("Required roles: %v, user role: %s", roles, userRole))
				return
			}

			next(w, r)
		})
	}
}

// OptionalAuth is a middleware that optionally validates JWT authentication
// If a valid token is provided, user information is added to context
// If no token or invalid token is provided, the request continues without authentication
func (jm *JWTMiddleware) OptionalAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract JWT token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			// No token provided, continue without authentication
			next(w, r)
			return
		}

		// Check if it's a Bearer token
		if !strings.HasPrefix(authHeader, "Bearer ") {
			// Invalid format, continue without authentication
			next(w, r)
			return
		}

		// Extract token
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			// Empty token, continue without authentication
			next(w, r)
			return
		}

		// Validate token
		claims, err := jm.jwtManager.ValidateToken(token)
		if err != nil {
			// Invalid token, continue without authentication
			next(w, r)
			return
		}

		// Add user information to request context
		ctx := r.Context()
		ctx = context.WithValue(ctx, UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, UserEmailKey, claims.Email)
		ctx = context.WithValue(ctx, UserRoleKey, claims.Role)
		ctx = context.WithValue(ctx, JWTClaimsKey, claims)

		// Call next handler with updated context
		next(w, r.WithContext(ctx))
	}
}

// Helper functions to extract user information from context

// GetUserID extracts user ID from request context
func GetUserID(ctx context.Context) string {
	if userID, ok := ctx.Value(UserIDKey).(string); ok {
		return userID
	}
	return ""
}

// GetUserEmail extracts user email from request context
func GetUserEmail(ctx context.Context) string {
	if email, ok := ctx.Value(UserEmailKey).(string); ok {
		return email
	}
	return ""
}

// GetUserRole extracts user role from request context
func GetUserRole(ctx context.Context) string {
	if role, ok := ctx.Value(UserRoleKey).(string); ok {
		return role
	}
	return ""
}

// GetJWTClaims extracts JWT claims from request context
func GetJWTClaims(ctx context.Context) *auth.JWTClaims {
	if claims, ok := ctx.Value(JWTClaimsKey).(*auth.JWTClaims); ok {
		return claims
	}
	return nil
}

// IsAuthenticated checks if the request is authenticated
func IsAuthenticated(ctx context.Context) bool {
	return GetUserID(ctx) != ""
}

// writeError writes an error response
func (jm *JWTMiddleware) writeError(w http.ResponseWriter, statusCode int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	response := map[string]interface{}{
		"error":   http.StatusText(statusCode),
		"message": message,
		"code":    code,
	}
	
	// Simple JSON encoding without external dependencies
	jsonStr := fmt.Sprintf(`{"error":"%s","message":"%s","code":"%s"}`, 
		response["error"], response["message"], response["code"])
	w.Write([]byte(jsonStr))
}
