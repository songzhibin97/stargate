package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/songzhibin97/stargate/internal/config"
)

func TestJWTAuthenticator_Authenticate(t *testing.T) {
	// Generate test RSA key pair (for future RSA tests)
	_, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	// Create test configuration with HMAC secret
	cfg := &config.JWTConfig{
		Secret:    "test-secret-key",
		Algorithm: "HS256",
		Issuer:    "test-issuer",
		Audience:  "test-audience",
	}

	// Create authenticator
	auth, err := NewJWTAuthenticator(cfg)
	if err != nil {
		t.Fatalf("Failed to create JWT authenticator: %v", err)
	}

	// Helper function to create test tokens
	createToken := func(claims jwt.MapClaims) string {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(cfg.Secret))
		if err != nil {
			t.Fatalf("Failed to sign token: %v", err)
		}
		return tokenString
	}

	tests := []struct {
		name           string
		setupRequest   func() *http.Request
		expectedAuth   bool
		expectedError  string
		expectedStatus int
	}{
		{
			name: "Valid JWT token",
			setupRequest: func() *http.Request {
				claims := jwt.MapClaims{
					"sub":  "user123",
					"iss":  "test-issuer",
					"aud":  "test-audience",
					"exp":  time.Now().Add(time.Hour).Unix(),
					"iat":  time.Now().Unix(),
					"name": "Test User",
					"email": "test@example.com",
					"roles": []string{"user", "admin"},
				}
				token := createToken(claims)
				
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "Bearer "+token)
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
			expectedError:  "JWT token not provided",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Invalid Authorization header format",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "InvalidFormat token")
				return req
			},
			expectedAuth:   false,
			expectedError:  "JWT token not provided",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Expired token",
			setupRequest: func() *http.Request {
				claims := jwt.MapClaims{
					"sub": "user123",
					"iss": "test-issuer",
					"aud": "test-audience",
					"exp": time.Now().Add(-time.Hour).Unix(), // Expired
					"iat": time.Now().Add(-2 * time.Hour).Unix(),
				}
				token := createToken(claims)
				
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "Bearer "+token)
				return req
			},
			expectedAuth:   false,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Invalid issuer",
			setupRequest: func() *http.Request {
				claims := jwt.MapClaims{
					"sub": "user123",
					"iss": "wrong-issuer",
					"aud": "test-audience",
					"exp": time.Now().Add(time.Hour).Unix(),
					"iat": time.Now().Unix(),
				}
				token := createToken(claims)
				
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "Bearer "+token)
				return req
			},
			expectedAuth:   false,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Invalid audience",
			setupRequest: func() *http.Request {
				claims := jwt.MapClaims{
					"sub": "user123",
					"iss": "test-issuer",
					"aud": "wrong-audience",
					"exp": time.Now().Add(time.Hour).Unix(),
					"iat": time.Now().Unix(),
				}
				token := createToken(claims)
				
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "Bearer "+token)
				return req
			},
			expectedAuth:   false,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Token with wrong signature",
			setupRequest: func() *http.Request {
				// Create token with different secret
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
					"sub": "user123",
					"iss": "test-issuer",
					"aud": "test-audience",
					"exp": time.Now().Add(time.Hour).Unix(),
					"iat": time.Now().Unix(),
				})
				tokenString, _ := token.SignedString([]byte("wrong-secret"))
				
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "Bearer "+tokenString)
				return req
			},
			expectedAuth:   false,
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

func TestJWTAuthenticator_GetName(t *testing.T) {
	cfg := &config.JWTConfig{
		Secret: "test-secret",
	}

	auth, err := NewJWTAuthenticator(cfg)
	if err != nil {
		t.Fatalf("Failed to create JWT authenticator: %v", err)
	}

	name := auth.GetName()
	if name != "jwt" {
		t.Errorf("Expected name='jwt', got %q", name)
	}
}

func TestJWTAuthenticator_ExtractToken(t *testing.T) {
	cfg := &config.JWTConfig{
		Secret: "test-secret",
	}

	auth, err := NewJWTAuthenticator(cfg)
	if err != nil {
		t.Fatalf("Failed to create JWT authenticator: %v", err)
	}

	tests := []struct {
		name           string
		authHeader     string
		expectedToken  string
	}{
		{
			name:          "Valid Bearer token",
			authHeader:    "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expectedToken: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
		},
		{
			name:          "Bearer with different case",
			authHeader:    "bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expectedToken: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
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

func TestJWTAuthenticator_CreateUserInfoFromClaims(t *testing.T) {
	cfg := &config.JWTConfig{
		Secret: "test-secret",
	}

	auth, err := NewJWTAuthenticator(cfg)
	if err != nil {
		t.Fatalf("Failed to create JWT authenticator: %v", err)
	}

	expiresAt := jwt.NewNumericDate(time.Now().Add(time.Hour))
	claims := &JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user123",
			ExpiresAt: expiresAt,
		},
		Email:       "test@example.com",
		Name:        "Test User",
		Username:    "testuser",
		Roles:       []string{"admin", "user"},
		Permissions: []string{"read", "write"},
		Groups:      []string{"developers", "admins"},
	}

	userInfo := auth.createUserInfoFromClaims(claims)

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
	if len(userInfo.Permissions) != 2 {
		t.Errorf("Expected 2 permissions, got %d", len(userInfo.Permissions))
	}
	if len(userInfo.Groups) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(userInfo.Groups))
	}
	if userInfo.ExpiresAt == nil {
		t.Error("Expected ExpiresAt to be set")
	}
}
