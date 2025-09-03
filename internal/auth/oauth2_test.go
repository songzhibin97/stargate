package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
)

func TestOAuth2Authenticator_Authenticate(t *testing.T) {
	// Create mock introspection server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and content type
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("Expected application/x-www-form-urlencoded content type")
		}

		// Parse form data
		if err := r.ParseForm(); err != nil {
			t.Fatalf("Failed to parse form: %v", err)
		}

		token := r.FormValue("token")
		
		// Mock responses based on token
		var response IntrospectionResponse
		switch token {
		case "valid-token":
			response = IntrospectionResponse{
				Active:      true,
				Sub:         "user123",
				Username:    "testuser",
				Email:       "test@example.com",
				Scope:       "read write",
				ClientID:    "test-client",
				TokenType:   "Bearer",
				Exp:         time.Now().Add(time.Hour).Unix(),
				Iat:         time.Now().Unix(),
				Roles:       []string{"user", "admin"},
				Permissions: []string{"read", "write"},
				Groups:      []string{"developers"},
			}
		case "expired-token":
			response = IntrospectionResponse{
				Active: true,
				Sub:    "user123",
				Exp:    time.Now().Add(-time.Hour).Unix(), // Expired
			}
		case "inactive-token":
			response = IntrospectionResponse{
				Active: false,
			}
		case "server-error":
			w.WriteHeader(http.StatusInternalServerError)
			return
		default:
			response = IntrospectionResponse{
				Active: false,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Create test configuration
	cfg := &config.OAuth2Config{
		IntrospectionURL: mockServer.URL,
		ClientID:         "test-client",
		ClientSecret:     "test-secret",
		Timeout:          5 * time.Second,
		MaxRetries:       2,
		RetryDelay:       100 * time.Millisecond,
	}

	// Create authenticator
	auth := NewOAuth2Authenticator(cfg)

	tests := []struct {
		name           string
		setupRequest   func() *http.Request
		expectedAuth   bool
		expectedError  string
		expectedStatus int
	}{
		{
			name: "Valid OAuth2 token",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "Bearer valid-token")
				return req
			},
			expectedAuth: true,
		},
		{
			name: "Missing Authorization header",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				return req
			},
			expectedAuth:   false,
			expectedError:  "OAuth 2.0 token not provided",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Invalid Authorization header format",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
				return req
			},
			expectedAuth:   false,
			expectedError:  "OAuth 2.0 token not provided",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Inactive token",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "Bearer inactive-token")
				return req
			},
			expectedAuth:   false,
			expectedError:  "Token is not active",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Expired token",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "Bearer expired-token")
				return req
			},
			expectedAuth:   false,
			expectedError:  "Token has expired",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Unknown token",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "Bearer unknown-token")
				return req
			},
			expectedAuth:   false,
			expectedError:  "Token is not active",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupRequest()
			result, err := auth.Authenticate(req)

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result.Authenticated != tt.expectedAuth {
				t.Errorf("Expected authenticated=%v, got %v", tt.expectedAuth, result.Authenticated)
			}

			if !tt.expectedAuth {
				if tt.expectedError != "" && result.Error != tt.expectedError {
					t.Errorf("Expected error=%q, got %q", tt.expectedError, result.Error)
				}
				if tt.expectedStatus != 0 && result.StatusCode != tt.expectedStatus {
					t.Errorf("Expected status=%d, got %d", tt.expectedStatus, result.StatusCode)
				}
			} else {
				if result.UserInfo == nil {
					t.Error("Expected UserInfo to be set for authenticated request")
				}
				if result.Claims == nil {
					t.Error("Expected Claims to be set for authenticated request")
				}
			}
		})
	}
}

func TestOAuth2Authenticator_GetName(t *testing.T) {
	cfg := &config.OAuth2Config{
		IntrospectionURL: "http://localhost:8080/introspect",
	}

	auth := NewOAuth2Authenticator(cfg)
	name := auth.GetName()

	if name != "oauth2" {
		t.Errorf("Expected name='oauth2', got %q", name)
	}
}

func TestOAuth2Authenticator_ExtractToken(t *testing.T) {
	cfg := &config.OAuth2Config{
		IntrospectionURL: "http://localhost:8080/introspect",
	}

	auth := NewOAuth2Authenticator(cfg)

	tests := []struct {
		name           string
		authHeader     string
		expectedToken  string
	}{
		{
			name:          "Valid Bearer token",
			authHeader:    "Bearer abc123def456",
			expectedToken: "abc123def456",
		},
		{
			name:          "Bearer with different case",
			authHeader:    "bearer xyz789",
			expectedToken: "xyz789",
		},
		{
			name:          "No Authorization header",
			authHeader:    "",
			expectedToken: "",
		},
		{
			name:          "Invalid format",
			authHeader:    "Basic dXNlcjpwYXNz",
			expectedToken: "",
		},
		{
			name:          "Bearer without token",
			authHeader:    "Bearer",
			expectedToken: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			token := auth.extractToken(req)
			if token != tt.expectedToken {
				t.Errorf("Expected token=%q, got %q", tt.expectedToken, token)
			}
		})
	}
}

func TestOAuth2Authenticator_CreateUserInfoFromIntrospection(t *testing.T) {
	cfg := &config.OAuth2Config{
		IntrospectionURL: "http://localhost:8080/introspect",
	}

	auth := NewOAuth2Authenticator(cfg)

	resp := &IntrospectionResponse{
		Active:      true,
		Sub:         "user123",
		Username:    "testuser",
		Email:       "test@example.com",
		Scope:       "read write admin",
		ClientID:    "test-client",
		Exp:         time.Now().Add(time.Hour).Unix(),
		Roles:       []string{"admin", "user"},
		Permissions: []string{"read", "write", "delete"},
		Groups:      []string{"developers", "admins"},
	}

	userInfo := auth.createUserInfoFromIntrospection(resp)

	if userInfo.ID != "user123" {
		t.Errorf("Expected ID='user123', got %q", userInfo.ID)
	}
	if userInfo.Username != "testuser" {
		t.Errorf("Expected Username='testuser', got %q", userInfo.Username)
	}
	if userInfo.Email != "test@example.com" {
		t.Errorf("Expected Email='test@example.com', got %q", userInfo.Email)
	}
	if len(userInfo.Roles) != 2 {
		t.Errorf("Expected 2 roles, got %d", len(userInfo.Roles))
	}
	if len(userInfo.Permissions) != 3 {
		t.Errorf("Expected 3 permissions, got %d", len(userInfo.Permissions))
	}
	if len(userInfo.Groups) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(userInfo.Groups))
	}
	if userInfo.ExpiresAt == nil {
		t.Error("Expected ExpiresAt to be set")
	}
	if userInfo.Metadata["scope"] != "read write admin" {
		t.Errorf("Expected scope metadata, got %q", userInfo.Metadata["scope"])
	}
	if userInfo.Metadata["client_id"] != "test-client" {
		t.Errorf("Expected client_id metadata, got %q", userInfo.Metadata["client_id"])
	}
}

func TestOAuth2Authenticator_IntrospectionToClaims(t *testing.T) {
	cfg := &config.OAuth2Config{
		IntrospectionURL: "http://localhost:8080/introspect",
	}

	auth := NewOAuth2Authenticator(cfg)

	resp := &IntrospectionResponse{
		Active:      true,
		Sub:         "user123",
		Username:    "testuser",
		Email:       "test@example.com",
		Scope:       "read write",
		ClientID:    "test-client",
		TokenType:   "Bearer",
		Exp:         1234567890,
		Iat:         1234567800,
		Roles:       []string{"user"},
		Permissions: []string{"read"},
		Groups:      []string{"developers"},
	}

	claims := auth.introspectionToClaims(resp)

	if claims["active"] != true {
		t.Errorf("Expected active=true, got %v", claims["active"])
	}
	if claims["sub"] != "user123" {
		t.Errorf("Expected sub='user123', got %v", claims["sub"])
	}
	if claims["username"] != "testuser" {
		t.Errorf("Expected username='testuser', got %v", claims["username"])
	}
	if claims["email"] != "test@example.com" {
		t.Errorf("Expected email='test@example.com', got %v", claims["email"])
	}
	if claims["scope"] != "read write" {
		t.Errorf("Expected scope='read write', got %v", claims["scope"])
	}
	if claims["client_id"] != "test-client" {
		t.Errorf("Expected client_id='test-client', got %v", claims["client_id"])
	}
	if claims["token_type"] != "Bearer" {
		t.Errorf("Expected token_type='Bearer', got %v", claims["token_type"])
	}
	if claims["exp"] != int64(1234567890) {
		t.Errorf("Expected exp=1234567890, got %v", claims["exp"])
	}
	if claims["iat"] != int64(1234567800) {
		t.Errorf("Expected iat=1234567800, got %v", claims["iat"])
	}
}
