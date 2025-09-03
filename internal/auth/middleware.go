package auth

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
)

// Middleware represents the authentication middleware
type Middleware struct {
	config        *config.AuthConfig
	authenticators map[AuthenticationMethod]Authenticator
	mu            sync.RWMutex
}

// NewMiddleware creates a new authentication middleware
func NewMiddleware(config *config.AuthConfig) *Middleware {
	m := &Middleware{
		config:        config,
		authenticators: make(map[AuthenticationMethod]Authenticator),
	}
	
	// Initialize authenticators based on configuration
	m.initializeAuthenticators()
	
	return m
}

// initializeAuthenticators initializes authenticators based on configuration
func (m *Middleware) initializeAuthenticators() {
	// Initialize API Key authenticator
	if m.config.APIKey.Header != "" || m.config.APIKey.Query != "" {
		apiKeyAuth := NewAPIKeyAuthenticator(&m.config.APIKey)
		m.authenticators[AuthMethodAPIKey] = apiKeyAuth
	}

	// Initialize JWT authenticator
	if m.config.JWT.Secret != "" || m.config.JWT.PublicKey != "" || m.config.JWT.JWKSURL != "" {
		jwtAuth, err := NewJWTAuthenticator(&m.config.JWT)
		if err != nil {
			log.Printf("Failed to initialize JWT authenticator: %v", err)
		} else {
			m.authenticators[AuthMethodJWT] = jwtAuth
		}
	}

	// Initialize OAuth2 authenticator
	if m.config.OAuth2.IntrospectionURL != "" {
		oauth2Auth := NewOAuth2Authenticator(&m.config.OAuth2)
		m.authenticators[AuthMethodOAuth2] = oauth2Auth
	}
}

// Handler returns the HTTP middleware handler
func (m *Middleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip authentication if disabled
			if !m.config.Enabled {
				next.ServeHTTP(w, r)
				return
			}
			
			// Try to authenticate the request
			authResult, err := m.authenticate(r)
			if err != nil {
				log.Printf("Authentication error: %v", err)
				m.handleAuthError(w, r, &AuthResult{
					Authenticated: false,
					Error:         "Internal authentication error",
					StatusCode:    http.StatusInternalServerError,
				})
				return
			}
			
			// Handle authentication result
			if !authResult.Authenticated {
				m.handleAuthError(w, r, authResult)
				return
			}
			
			// Set authentication context in request
			ctx := r.Context()
			if authResult.UserInfo != nil {
				ctx = SetUserInContext(ctx, authResult.UserInfo)
			}
			if authResult.Consumer != nil {
				ctx = SetConsumerInContext(ctx, authResult.Consumer)
			}
			if authResult.Claims != nil {
				ctx = SetClaimsInContext(ctx, authResult.Claims)
			}
			
			// Set authentication method
			if method := m.getAuthMethod(r); method != "" {
				ctx = SetAuthMethodInContext(ctx, method)
			}
			
			// Add authentication headers for upstream services
			m.addUpstreamHeaders(w, r, authResult)
			
			// Continue to next handler with updated context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// authenticate attempts to authenticate the request using available authenticators
func (m *Middleware) authenticate(r *http.Request) (*AuthResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var lastError error
	var lastResult *AuthResult
	
	// Try each authenticator in order of preference
	authMethods := []AuthenticationMethod{
		AuthMethodAPIKey,
		AuthMethodJWT,
		AuthMethodOAuth2,
	}
	
	for _, method := range authMethods {
		authenticator, exists := m.authenticators[method]
		if !exists {
			continue
		}
		
		// Check if this authentication method is applicable to the request
		if !m.isAuthMethodApplicable(r, method) {
			continue
		}
		
		// Try to authenticate
		result, err := authenticator.Authenticate(r)
		if err != nil {
			lastError = err
			continue
		}
		
		// If authentication succeeded, return the result
		if result.Authenticated {
			return result, nil
		}
		
		// Keep track of the last result for error handling
		lastResult = result
	}
	
	// If no authenticator succeeded, return the last result or error
	if lastResult != nil {
		return lastResult, nil
	}
	
	if lastError != nil {
		return nil, lastError
	}
	
	// No credentials provided
	return &AuthResult{
		Authenticated: false,
		Error:         "No valid credentials provided",
		StatusCode:    http.StatusUnauthorized,
	}, nil
}

// isAuthMethodApplicable checks if an authentication method is applicable to the request
func (m *Middleware) isAuthMethodApplicable(r *http.Request, method AuthenticationMethod) bool {
	switch method {
	case AuthMethodAPIKey:
		// Check if API key is present in headers or query
		if m.config.APIKey.Header != "" && r.Header.Get(m.config.APIKey.Header) != "" {
			return true
		}
		if m.config.APIKey.Query != "" && r.URL.Query().Get(m.config.APIKey.Query) != "" {
			return true
		}
		return false
		
	case AuthMethodJWT:
		// Check if Authorization header with Bearer token is present
		authHeader := r.Header.Get("Authorization")
		return strings.HasPrefix(authHeader, "Bearer ")
		
	case AuthMethodOAuth2:
		// Check if Authorization header with Bearer token is present
		authHeader := r.Header.Get("Authorization")
		return strings.HasPrefix(authHeader, "Bearer ")
		
	default:
		return false
	}
}

// getAuthMethod determines which authentication method was used
func (m *Middleware) getAuthMethod(r *http.Request) string {
	// Check API key
	if m.config.APIKey.Header != "" && r.Header.Get(m.config.APIKey.Header) != "" {
		return string(AuthMethodAPIKey)
	}
	if m.config.APIKey.Query != "" && r.URL.Query().Get(m.config.APIKey.Query) != "" {
		return string(AuthMethodAPIKey)
	}
	
	// Check JWT/OAuth2
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		// This could be JWT or OAuth2 - would need to inspect the token to determine
		return string(AuthMethodJWT) // Default to JWT for now
	}
	
	return ""
}

// handleAuthError handles authentication errors
func (m *Middleware) handleAuthError(w http.ResponseWriter, r *http.Request, result *AuthResult) {
	statusCode := result.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusUnauthorized
	}
	
	// Set additional headers if specified
	if result.Headers != nil {
		for key, value := range result.Headers {
			w.Header().Set(key, value)
		}
	}
	
	// Set WWW-Authenticate header for 401 responses
	if statusCode == http.StatusUnauthorized {
		m.setWWWAuthenticateHeader(w)
	}
	
	// Create error response
	errorResponse := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    "AUTHENTICATION_FAILED",
			"message": result.Error,
		},
		"timestamp": fmt.Sprintf("%d", time.Now().Unix()),
		"path": r.URL.Path,
	}
	
	// Set content type and status code
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	// Write error response
	if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
		log.Printf("Failed to write authentication error response: %v", err)
	}
	
	// Log authentication failure
	log.Printf("Authentication failed for %s %s: %s", r.Method, r.URL.Path, result.Error)
}

// setWWWAuthenticateHeader sets the WWW-Authenticate header
func (m *Middleware) setWWWAuthenticateHeader(w http.ResponseWriter) {
	var challenges []string
	
	// Add API key challenge if configured
	if m.config.APIKey.Header != "" || m.config.APIKey.Query != "" {
		challenges = append(challenges, "ApiKey")
	}
	
	// Add Bearer challenge for JWT/OAuth2
	if m.config.JWT.Secret != "" {
		challenges = append(challenges, "Bearer")
	}
	
	if len(challenges) > 0 {
		w.Header().Set("WWW-Authenticate", strings.Join(challenges, ", "))
	}
}

// addUpstreamHeaders adds authentication headers for upstream services
func (m *Middleware) addUpstreamHeaders(w http.ResponseWriter, r *http.Request, result *AuthResult) {
	// Add user information headers
	if result.UserInfo != nil {
		r.Header.Set("X-User-ID", result.UserInfo.ID)
		r.Header.Set("X-User-Name", result.UserInfo.Username)
		
		if result.UserInfo.Email != "" {
			r.Header.Set("X-User-Email", result.UserInfo.Email)
		}
		
		if len(result.UserInfo.Roles) > 0 {
			r.Header.Set("X-User-Roles", strings.Join(result.UserInfo.Roles, ","))
		}
		
		if len(result.UserInfo.Groups) > 0 {
			r.Header.Set("X-User-Groups", strings.Join(result.UserInfo.Groups, ","))
		}
	}
	
	// Add consumer information headers
	if result.Consumer != nil {
		r.Header.Set("X-Consumer-ID", result.Consumer.ID)
		r.Header.Set("X-Consumer-Name", result.Consumer.Name)
	}
	
	// Add authentication method header
	if method := m.getAuthMethod(r); method != "" {
		r.Header.Set("X-Auth-Method", method)
	}
}

// AddAuthenticator adds a custom authenticator
func (m *Middleware) AddAuthenticator(method AuthenticationMethod, authenticator Authenticator) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.authenticators[method] = authenticator
}

// RemoveAuthenticator removes an authenticator
func (m *Middleware) RemoveAuthenticator(method AuthenticationMethod) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	delete(m.authenticators, method)
}

// GetAuthenticator gets an authenticator by method
func (m *Middleware) GetAuthenticator(method AuthenticationMethod) (Authenticator, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	authenticator, exists := m.authenticators[method]
	return authenticator, exists
}
