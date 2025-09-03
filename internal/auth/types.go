package auth

import (
	"context"
	"net/http"
	"time"
)

// Authenticator defines the interface for authentication providers
type Authenticator interface {
	// Authenticate authenticates a request and returns the result
	Authenticate(r *http.Request) (*AuthResult, error)
	
	// GetName returns the name of the authenticator
	GetName() string
}

// AuthResult represents the result of an authentication attempt
type AuthResult struct {
	// Authenticated indicates if the request was successfully authenticated
	Authenticated bool `json:"authenticated"`
	
	// UserInfo contains information about the authenticated user
	UserInfo *UserInfo `json:"user_info,omitempty"`
	
	// Consumer contains information about the API consumer (for API key auth)
	Consumer *Consumer `json:"consumer,omitempty"`
	
	// Claims contains JWT claims (for JWT auth)
	Claims map[string]interface{} `json:"claims,omitempty"`
	
	// Error contains error message if authentication failed
	Error string `json:"error,omitempty"`
	
	// StatusCode is the HTTP status code to return on failure
	StatusCode int `json:"status_code,omitempty"`
	
	// Headers contains additional headers to set
	Headers map[string]string `json:"headers,omitempty"`
	
	// Metadata contains additional authentication metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// UserInfo represents authenticated user information
type UserInfo struct {
	// ID is the unique identifier for the user
	ID string `json:"id"`
	
	// Username is the user's username
	Username string `json:"username"`
	
	// Email is the user's email address
	Email string `json:"email,omitempty"`
	
	// Roles contains the user's roles
	Roles []string `json:"roles,omitempty"`
	
	// Permissions contains the user's permissions
	Permissions []string `json:"permissions,omitempty"`
	
	// Groups contains the user's groups
	Groups []string `json:"groups,omitempty"`
	
	// Metadata contains additional user metadata
	Metadata map[string]string `json:"metadata,omitempty"`
	
	// ExpiresAt indicates when the authentication expires
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// AuthContext represents authentication context
type AuthContext struct {
	// UserInfo contains authenticated user information
	UserInfo *UserInfo
	
	// Consumer contains API consumer information
	Consumer *Consumer
	
	// Claims contains JWT claims
	Claims map[string]interface{}
	
	// AuthMethod indicates the authentication method used
	AuthMethod string
	
	// AuthenticatedAt indicates when authentication occurred
	AuthenticatedAt time.Time
}

// AuthContextKey is the key used to store auth context in request context
type AuthContextKey string

const (
	// AuthContextKeyUser is the key for user info in context
	AuthContextKeyUser AuthContextKey = "auth_user"
	
	// AuthContextKeyConsumer is the key for consumer info in context
	AuthContextKeyConsumer AuthContextKey = "auth_consumer"
	
	// AuthContextKeyClaims is the key for JWT claims in context
	AuthContextKeyClaims AuthContextKey = "auth_claims"
	
	// AuthContextKeyMethod is the key for auth method in context
	AuthContextKeyMethod AuthContextKey = "auth_method"
)

// GetUserFromContext extracts user info from request context
func GetUserFromContext(ctx context.Context) (*UserInfo, bool) {
	user, ok := ctx.Value(AuthContextKeyUser).(*UserInfo)
	return user, ok
}

// GetConsumerFromContext extracts consumer info from request context
func GetConsumerFromContext(ctx context.Context) (*Consumer, bool) {
	consumer, ok := ctx.Value(AuthContextKeyConsumer).(*Consumer)
	return consumer, ok
}

// GetClaimsFromContext extracts JWT claims from request context
func GetClaimsFromContext(ctx context.Context) (map[string]interface{}, bool) {
	claims, ok := ctx.Value(AuthContextKeyClaims).(map[string]interface{})
	return claims, ok
}

// GetAuthMethodFromContext extracts auth method from request context
func GetAuthMethodFromContext(ctx context.Context) (string, bool) {
	method, ok := ctx.Value(AuthContextKeyMethod).(string)
	return method, ok
}

// SetUserInContext sets user info in request context
func SetUserInContext(ctx context.Context, user *UserInfo) context.Context {
	return context.WithValue(ctx, AuthContextKeyUser, user)
}

// SetConsumerInContext sets consumer info in request context
func SetConsumerInContext(ctx context.Context, consumer *Consumer) context.Context {
	return context.WithValue(ctx, AuthContextKeyConsumer, consumer)
}

// SetClaimsInContext sets JWT claims in request context
func SetClaimsInContext(ctx context.Context, claims map[string]interface{}) context.Context {
	return context.WithValue(ctx, AuthContextKeyClaims, claims)
}

// SetAuthMethodInContext sets auth method in request context
func SetAuthMethodInContext(ctx context.Context, method string) context.Context {
	return context.WithValue(ctx, AuthContextKeyMethod, method)
}

// AuthError represents an authentication error
type AuthError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	StatusCode int    `json:"status_code"`
}

// Error implements the error interface
func (e *AuthError) Error() string {
	return e.Message
}

// Common authentication error codes
const (
	AuthErrorCodeMissingCredentials   = "MISSING_CREDENTIALS"
	AuthErrorCodeInvalidCredentials   = "INVALID_CREDENTIALS"
	AuthErrorCodeExpiredCredentials   = "EXPIRED_CREDENTIALS"
	AuthErrorCodeDisabledAccount      = "DISABLED_ACCOUNT"
	AuthErrorCodeInsufficientPrivileges = "INSUFFICIENT_PRIVILEGES"
	AuthErrorCodeIPNotWhitelisted     = "IP_NOT_WHITELISTED"
	AuthErrorCodeRateLimitExceeded    = "RATE_LIMIT_EXCEEDED"
)

// NewAuthError creates a new authentication error
func NewAuthError(code, message string, statusCode int) *AuthError {
	return &AuthError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

// AuthenticationMethod represents different authentication methods
type AuthenticationMethod string

const (
	// AuthMethodAPIKey represents API key authentication
	AuthMethodAPIKey AuthenticationMethod = "api_key"
	
	// AuthMethodJWT represents JWT authentication
	AuthMethodJWT AuthenticationMethod = "jwt"
	
	// AuthMethodOAuth2 represents OAuth 2.0 authentication
	AuthMethodOAuth2 AuthenticationMethod = "oauth2"
	
	// AuthMethodBasic represents HTTP Basic authentication
	AuthMethodBasic AuthenticationMethod = "basic"
)

// String returns the string representation of the authentication method
func (am AuthenticationMethod) String() string {
	return string(am)
}

// AuthConfig represents authentication configuration for a route or service
type AuthConfig struct {
	// Enabled indicates if authentication is enabled
	Enabled bool `json:"enabled"`
	
	// Methods specifies which authentication methods are allowed
	Methods []AuthenticationMethod `json:"methods"`
	
	// Required indicates if authentication is required (vs optional)
	Required bool `json:"required"`
	
	// FailureMode specifies how to handle authentication failures
	FailureMode AuthFailureMode `json:"failure_mode"`
	
	// Headers specifies which headers to pass to upstream services
	PassHeaders []string `json:"pass_headers,omitempty"`
	
	// HideCredentials indicates if credentials should be hidden from upstream
	HideCredentials bool `json:"hide_credentials"`
}

// AuthFailureMode represents how to handle authentication failures
type AuthFailureMode string

const (
	// AuthFailureModeReject rejects the request with an error
	AuthFailureModeReject AuthFailureMode = "reject"
	
	// AuthFailureModePassthrough allows the request to continue without authentication
	AuthFailureModePassthrough AuthFailureMode = "passthrough"
)

// String returns the string representation of the failure mode
func (afm AuthFailureMode) String() string {
	return string(afm)
}
