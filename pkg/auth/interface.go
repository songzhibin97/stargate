package auth

import (
	"context"
	"net/http"
	"time"
)

// Authenticator defines the interface for authentication
type Authenticator interface {
	// Authenticate authenticates a request
	Authenticate(ctx context.Context, req *http.Request) (*Principal, error)
	
	// Name returns the authenticator name
	Name() string
	
	// Type returns the authenticator type
	Type() string
	
	// Configure configures the authenticator
	Configure(config map[string]interface{}) error
}

// Principal represents an authenticated user/service
type Principal struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        PrincipalType     `json:"type"`
	Attributes  map[string]string `json:"attributes,omitempty"`
	Permissions []string          `json:"permissions,omitempty"`
	Roles       []string          `json:"roles,omitempty"`
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
	IssuedAt    time.Time         `json:"issued_at"`
}

// PrincipalType represents the type of principal
type PrincipalType string

const (
	PrincipalTypeUser    PrincipalType = "user"
	PrincipalTypeService PrincipalType = "service"
	PrincipalTypeSystem  PrincipalType = "system"
)

// Authorizer defines the interface for authorization
type Authorizer interface {
	// Authorize checks if a principal is authorized for a resource/action
	Authorize(ctx context.Context, principal *Principal, resource string, action string) error
	
	// Name returns the authorizer name
	Name() string
	
	// Configure configures the authorizer
	Configure(config map[string]interface{}) error
}

// TokenProvider defines the interface for token operations
type TokenProvider interface {
	// GenerateToken generates a new token
	GenerateToken(ctx context.Context, principal *Principal) (*Token, error)
	
	// ValidateToken validates a token
	ValidateToken(ctx context.Context, tokenString string) (*Token, error)
	
	// RefreshToken refreshes a token
	RefreshToken(ctx context.Context, refreshToken string) (*Token, error)
	
	// RevokeToken revokes a token
	RevokeToken(ctx context.Context, tokenString string) error
	
	// Name returns the provider name
	Name() string
}

// Token represents an authentication token
type Token struct {
	AccessToken  string     `json:"access_token"`
	RefreshToken string     `json:"refresh_token,omitempty"`
	TokenType    string     `json:"token_type"`
	ExpiresIn    int64      `json:"expires_in"`
	ExpiresAt    time.Time  `json:"expires_at"`
	IssuedAt     time.Time  `json:"issued_at"`
	Principal    *Principal `json:"principal,omitempty"`
}

// CredentialStore defines the interface for credential storage
type CredentialStore interface {
	// StoreCredential stores a credential
	StoreCredential(ctx context.Context, id string, credential *Credential) error
	
	// GetCredential retrieves a credential
	GetCredential(ctx context.Context, id string) (*Credential, error)
	
	// DeleteCredential deletes a credential
	DeleteCredential(ctx context.Context, id string) error
	
	// ListCredentials lists credentials
	ListCredentials(ctx context.Context, filter *CredentialFilter) ([]*Credential, error)
	
	// UpdateCredential updates a credential
	UpdateCredential(ctx context.Context, id string, credential *Credential) error
}

// Credential represents stored credentials
type Credential struct {
	ID          string                 `json:"id"`
	Type        CredentialType         `json:"type"`
	Data        map[string]interface{} `json:"data"`
	Metadata    map[string]string      `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
	IsActive    bool                   `json:"is_active"`
	Description string                 `json:"description,omitempty"`
}

// CredentialType represents the type of credential
type CredentialType string

const (
	CredentialTypePassword CredentialType = "password"
	CredentialTypeAPIKey   CredentialType = "api_key"
	CredentialTypeCert     CredentialType = "certificate"
	CredentialTypeOAuth2   CredentialType = "oauth2"
	CredentialTypeJWT      CredentialType = "jwt"
)

// CredentialFilter represents filters for credential queries
type CredentialFilter struct {
	Type      *CredentialType `json:"type,omitempty"`
	IsActive  *bool           `json:"is_active,omitempty"`
	CreatedBy string          `json:"created_by,omitempty"`
	Limit     int             `json:"limit,omitempty"`
	Offset    int             `json:"offset,omitempty"`
}

// SessionManager defines the interface for session management
type SessionManager interface {
	// CreateSession creates a new session
	CreateSession(ctx context.Context, principal *Principal) (*Session, error)
	
	// GetSession retrieves a session
	GetSession(ctx context.Context, sessionID string) (*Session, error)
	
	// UpdateSession updates a session
	UpdateSession(ctx context.Context, session *Session) error
	
	// DeleteSession deletes a session
	DeleteSession(ctx context.Context, sessionID string) error
	
	// CleanupExpiredSessions cleans up expired sessions
	CleanupExpiredSessions(ctx context.Context) error
}

// Session represents a user session
type Session struct {
	ID        string            `json:"id"`
	Principal *Principal        `json:"principal"`
	Data      map[string]string `json:"data,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	ExpiresAt time.Time         `json:"expires_at"`
	IsActive  bool              `json:"is_active"`
}

// Middleware defines the interface for authentication middleware
type Middleware interface {
	// Handle wraps an HTTP handler with authentication
	Handle(next http.Handler) http.Handler
	
	// Configure configures the middleware
	Configure(config map[string]interface{}) error
	
	// Name returns the middleware name
	Name() string
}

// Manager defines the interface for authentication management
type Manager interface {
	// RegisterAuthenticator registers an authenticator
	RegisterAuthenticator(name string, auth Authenticator) error
	
	// GetAuthenticator gets an authenticator by name
	GetAuthenticator(name string) (Authenticator, error)
	
	// RegisterAuthorizer registers an authorizer
	RegisterAuthorizer(name string, authz Authorizer) error
	
	// GetAuthorizer gets an authorizer by name
	GetAuthorizer(name string) (Authorizer, error)
	
	// RegisterTokenProvider registers a token provider
	RegisterTokenProvider(name string, provider TokenProvider) error
	
	// GetTokenProvider gets a token provider by name
	GetTokenProvider(name string) (TokenProvider, error)
	
	// CreateMiddleware creates authentication middleware
	CreateMiddleware(config MiddlewareConfig) (Middleware, error)
}

// MiddlewareConfig represents middleware configuration
type MiddlewareConfig struct {
	Authenticators []string               `json:"authenticators"`
	Authorizers    []string               `json:"authorizers"`
	TokenProvider  string                 `json:"token_provider,omitempty"`
	Options        map[string]interface{} `json:"options,omitempty"`
	SkipPaths      []string               `json:"skip_paths,omitempty"`
	RequiredRoles  []string               `json:"required_roles,omitempty"`
	RequiredPerms  []string               `json:"required_permissions,omitempty"`
}
