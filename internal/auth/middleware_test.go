package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/songzhibin97/stargate/internal/config"
)

func TestMiddleware_Handler(t *testing.T) {
	// Create test configuration
	cfg := &config.AuthConfig{
		Enabled: true,
		APIKey: config.APIKeyConfig{
			Header: "X-API-Key",
			Query:  "api_key",
			Keys:   []string{"valid-key"},
		},
	}

	// Create middleware
	middleware := NewMiddleware(cfg)

	// Create a test handler that checks authentication context
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if user info is in context
		user, hasUser := GetUserFromContext(r.Context())
		consumer, hasConsumer := GetConsumerFromContext(r.Context())
		method, hasMethod := GetAuthMethodFromContext(r.Context())

		if hasUser && user != nil {
			w.Header().Set("X-Test-User-ID", user.ID)
		}
		if hasConsumer && consumer != nil {
			w.Header().Set("X-Test-Consumer-ID", consumer.ID)
		}
		if hasMethod {
			w.Header().Set("X-Test-Auth-Method", method)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with auth middleware
	handler := middleware.Handler()(testHandler)

	tests := []struct {
		name           string
		setupRequest   func() *http.Request
		expectedStatus int
		checkResponse  func(t *testing.T, resp *httptest.ResponseRecorder)
	}{
		{
			name: "Valid API key in header",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-API-Key", "valid-key")
				return req
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				if resp.Header().Get("X-Test-User-ID") == "" {
					t.Error("Expected X-Test-User-ID header to be set")
				}
				if resp.Header().Get("X-Test-Consumer-ID") == "" {
					t.Error("Expected X-Test-Consumer-ID header to be set")
				}
				if resp.Header().Get("X-Test-Auth-Method") != "api_key" {
					t.Errorf("Expected auth method 'api_key', got %q", resp.Header().Get("X-Test-Auth-Method"))
				}
			},
		},
		{
			name: "Valid API key in query parameter",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test?api_key=valid-key", nil)
				return req
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				if resp.Header().Get("X-Test-User-ID") == "" {
					t.Error("Expected X-Test-User-ID header to be set")
				}
			},
		},
		{
			name: "Missing API key",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				return req
			},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				contentType := resp.Header().Get("Content-Type")
				if contentType != "application/json" {
					t.Errorf("Expected Content-Type 'application/json', got %q", contentType)
				}
			},
		},
		{
			name: "Invalid API key",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-API-Key", "invalid-key")
				return req
			},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				contentType := resp.Header().Get("Content-Type")
				if contentType != "application/json" {
					t.Errorf("Expected Content-Type 'application/json', got %q", contentType)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupRequest()
			resp := httptest.NewRecorder()

			handler.ServeHTTP(resp, req)

			if resp.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.Code)
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}
		})
	}
}

func TestMiddleware_DisabledAuth(t *testing.T) {
	// Create configuration with auth disabled
	cfg := &config.AuthConfig{
		Enabled: false,
		APIKey: config.APIKeyConfig{
			Header: "X-API-Key",
			Keys:   []string{"valid-key"},
		},
	}

	// Create middleware
	middleware := NewMiddleware(cfg)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with auth middleware
	handler := middleware.Handler()(testHandler)

	// Test request without API key (should pass through)
	req := httptest.NewRequest("GET", "/test", nil)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, resp.Code)
	}

	if resp.Body.String() != "OK" {
		t.Errorf("Expected body 'OK', got %q", resp.Body.String())
	}
}

func TestAuthContext(t *testing.T) {
	ctx := context.Background()

	// Test UserInfo context
	user := &UserInfo{
		ID:       "user-123",
		Username: "testuser",
		Email:    "test@example.com",
	}

	ctx = SetUserInContext(ctx, user)
	retrievedUser, hasUser := GetUserFromContext(ctx)

	if !hasUser {
		t.Error("Expected user to be in context")
	}

	if retrievedUser.ID != user.ID {
		t.Errorf("Expected user ID %q, got %q", user.ID, retrievedUser.ID)
	}

	// Test Consumer context
	consumer := &Consumer{
		ID:   "consumer-123",
		Name: "Test Consumer",
	}

	ctx = SetConsumerInContext(ctx, consumer)
	retrievedConsumer, hasConsumer := GetConsumerFromContext(ctx)

	if !hasConsumer {
		t.Error("Expected consumer to be in context")
	}

	if retrievedConsumer.ID != consumer.ID {
		t.Errorf("Expected consumer ID %q, got %q", consumer.ID, retrievedConsumer.ID)
	}

	// Test Claims context
	claims := map[string]interface{}{
		"sub": "user-123",
		"iss": "test-issuer",
	}

	ctx = SetClaimsInContext(ctx, claims)
	retrievedClaims, hasClaims := GetClaimsFromContext(ctx)

	if !hasClaims {
		t.Error("Expected claims to be in context")
	}

	if retrievedClaims["sub"] != claims["sub"] {
		t.Errorf("Expected sub claim %q, got %q", claims["sub"], retrievedClaims["sub"])
	}

	// Test Auth Method context
	method := "api_key"
	ctx = SetAuthMethodInContext(ctx, method)
	retrievedMethod, hasMethod := GetAuthMethodFromContext(ctx)

	if !hasMethod {
		t.Error("Expected auth method to be in context")
	}

	if retrievedMethod != method {
		t.Errorf("Expected auth method %q, got %q", method, retrievedMethod)
	}
}

func TestMiddleware_AddRemoveAuthenticator(t *testing.T) {
	cfg := &config.AuthConfig{
		Enabled: true,
	}

	middleware := NewMiddleware(cfg)

	// Create a mock authenticator
	mockAuth := &MockAuthenticator{name: "mock"}

	// Add authenticator
	middleware.AddAuthenticator(AuthMethodAPIKey, mockAuth)

	// Get authenticator
	retrieved, exists := middleware.GetAuthenticator(AuthMethodAPIKey)
	if !exists {
		t.Error("Expected authenticator to exist")
	}

	if retrieved.GetName() != "mock" {
		t.Errorf("Expected authenticator name 'mock', got %q", retrieved.GetName())
	}

	// Remove authenticator
	middleware.RemoveAuthenticator(AuthMethodAPIKey)

	// Verify removal
	_, exists = middleware.GetAuthenticator(AuthMethodAPIKey)
	if exists {
		t.Error("Expected authenticator to be removed")
	}
}

// MockAuthenticator is a mock implementation for testing
type MockAuthenticator struct {
	name string
}

func (m *MockAuthenticator) Authenticate(r *http.Request) (*AuthResult, error) {
	return &AuthResult{
		Authenticated: true,
		UserInfo: &UserInfo{
			ID:       "mock-user",
			Username: "mockuser",
		},
	}, nil
}

func (m *MockAuthenticator) GetName() string {
	return m.name
}

func TestAuthError(t *testing.T) {
	err := NewAuthError(AuthErrorCodeInvalidCredentials, "Invalid credentials", http.StatusUnauthorized)

	if err.Code != AuthErrorCodeInvalidCredentials {
		t.Errorf("Expected code %q, got %q", AuthErrorCodeInvalidCredentials, err.Code)
	}

	if err.Message != "Invalid credentials" {
		t.Errorf("Expected message 'Invalid credentials', got %q", err.Message)
	}

	if err.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status code %d, got %d", http.StatusUnauthorized, err.StatusCode)
	}

	if err.Error() != "Invalid credentials" {
		t.Errorf("Expected error string 'Invalid credentials', got %q", err.Error())
	}
}

func TestAuthenticationMethod_String(t *testing.T) {
	tests := []struct {
		method   AuthenticationMethod
		expected string
	}{
		{AuthMethodAPIKey, "api_key"},
		{AuthMethodJWT, "jwt"},
		{AuthMethodOAuth2, "oauth2"},
		{AuthMethodBasic, "basic"},
	}

	for _, tt := range tests {
		t.Run(string(tt.method), func(t *testing.T) {
			if tt.method.String() != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, tt.method.String())
			}
		})
	}
}

func TestAuthFailureMode_String(t *testing.T) {
	tests := []struct {
		mode     AuthFailureMode
		expected string
	}{
		{AuthFailureModeReject, "reject"},
		{AuthFailureModePassthrough, "passthrough"},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			if tt.mode.String() != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, tt.mode.String())
			}
		})
	}
}
