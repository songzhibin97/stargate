package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
)

// OAuth2Authenticator handles OAuth 2.0 token introspection authentication
type OAuth2Authenticator struct {
	config     *config.OAuth2Config
	httpClient *http.Client
	mu         sync.RWMutex
}

// IntrospectionResponse represents the response from OAuth 2.0 introspection endpoint
type IntrospectionResponse struct {
	// Required fields (RFC 7662)
	Active bool `json:"active"`
	
	// Optional fields
	Scope     string `json:"scope,omitempty"`
	ClientID  string `json:"client_id,omitempty"`
	Username  string `json:"username,omitempty"`
	TokenType string `json:"token_type,omitempty"`
	Exp       int64  `json:"exp,omitempty"`
	Iat       int64  `json:"iat,omitempty"`
	Nbf       int64  `json:"nbf,omitempty"`
	Sub       string `json:"sub,omitempty"`
	Aud       string `json:"aud,omitempty"`
	Iss       string `json:"iss,omitempty"`
	Jti       string `json:"jti,omitempty"`
	
	// Extension fields
	Email       string                 `json:"email,omitempty"`
	Name        string                 `json:"name,omitempty"`
	Roles       []string               `json:"roles,omitempty"`
	Permissions []string               `json:"permissions,omitempty"`
	Groups      []string               `json:"groups,omitempty"`
	Extra       map[string]interface{} `json:"-"`
}

// TokenCache represents a cached token introspection result
type TokenCache struct {
	response  *IntrospectionResponse
	expiresAt time.Time
}

// OAuth2TokenCache caches introspection results
type OAuth2TokenCache struct {
	cache map[string]*TokenCache
	mu    sync.RWMutex
}

// NewOAuth2Authenticator creates a new OAuth 2.0 authenticator
func NewOAuth2Authenticator(config *config.OAuth2Config) *OAuth2Authenticator {
	// Set defaults
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = time.Second
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = 5 * time.Minute
	}

	return &OAuth2Authenticator{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Authenticate authenticates a request using OAuth 2.0 token introspection
func (o *OAuth2Authenticator) Authenticate(r *http.Request) (*AuthResult, error) {
	// Extract Bearer token from Authorization header
	token := o.extractToken(r)
	if token == "" {
		return &AuthResult{
			Authenticated: false,
			Error:         "OAuth 2.0 token not provided",
			StatusCode:    http.StatusUnauthorized,
		}, nil
	}
	
	// Introspect the token
	introspectionResp, err := o.introspectToken(token)
	if err != nil {
		return &AuthResult{
			Authenticated: false,
			Error:         fmt.Sprintf("Token introspection failed: %v", err),
			StatusCode:    http.StatusInternalServerError,
		}, nil
	}
	
	// Check if token is active
	if !introspectionResp.Active {
		return &AuthResult{
			Authenticated: false,
			Error:         "Token is not active",
			StatusCode:    http.StatusUnauthorized,
		}, nil
	}
	
	// Validate token expiration
	if introspectionResp.Exp > 0 && time.Now().Unix() > introspectionResp.Exp {
		return &AuthResult{
			Authenticated: false,
			Error:         "Token has expired",
			StatusCode:    http.StatusUnauthorized,
		}, nil
	}
	
	// Validate not before time
	if introspectionResp.Nbf > 0 && time.Now().Unix() < introspectionResp.Nbf {
		return &AuthResult{
			Authenticated: false,
			Error:         "Token not valid yet",
			StatusCode:    http.StatusUnauthorized,
		}, nil
	}
	
	// Create user info from introspection response
	userInfo := o.createUserInfoFromIntrospection(introspectionResp)
	
	// Convert introspection response to claims map
	claims := o.introspectionToClaims(introspectionResp)
	
	return &AuthResult{
		Authenticated: true,
		UserInfo:      userInfo,
		Claims:        claims,
	}, nil
}

// extractToken extracts Bearer token from Authorization header
func (o *OAuth2Authenticator) extractToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}
	
	// Check for Bearer token
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return ""
	}
	
	return parts[1]
}

// introspectToken performs token introspection using RFC 7662
func (o *OAuth2Authenticator) introspectToken(token string) (*IntrospectionResponse, error) {
	// Prepare introspection request
	data := url.Values{}
	data.Set("token", token)
	
	// Add token type hint if configured
	if o.config.TokenTypeHint != "" {
		data.Set("token_type_hint", o.config.TokenTypeHint)
	}
	
	// Create HTTP request
	req, err := http.NewRequest("POST", o.config.IntrospectionURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create introspection request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	
	// Add custom headers
	for key, value := range o.config.Headers {
		req.Header.Set(key, value)
	}
	
	// Set client authentication
	if o.config.ClientID != "" && o.config.ClientSecret != "" {
		req.SetBasicAuth(o.config.ClientID, o.config.ClientSecret)
	}
	
	// Add timeout context
	ctx, cancel := context.WithTimeout(context.Background(), o.config.Timeout)
	defer cancel()
	req = req.WithContext(ctx)
	
	// Perform request with retries
	var resp *http.Response
	var lastErr error
	
	for attempt := 0; attempt <= o.config.MaxRetries; attempt++ {
		resp, lastErr = o.httpClient.Do(req)
		if lastErr == nil && resp.StatusCode < 500 {
			break // Success or client error (don't retry)
		}
		
		if resp != nil {
			resp.Body.Close()
		}
		
		if attempt < o.config.MaxRetries {
			time.Sleep(o.config.RetryDelay * time.Duration(attempt+1))
		}
	}
	
	if lastErr != nil {
		return nil, fmt.Errorf("introspection request failed after %d attempts: %w", o.config.MaxRetries+1, lastErr)
	}
	defer resp.Body.Close()
	
	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("introspection endpoint returned status %d", resp.StatusCode)
	}
	
	// Parse response
	var introspectionResp IntrospectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&introspectionResp); err != nil {
		return nil, fmt.Errorf("failed to decode introspection response: %w", err)
	}
	
	return &introspectionResp, nil
}

// createUserInfoFromIntrospection creates UserInfo from introspection response
func (o *OAuth2Authenticator) createUserInfoFromIntrospection(resp *IntrospectionResponse) *UserInfo {
	userInfo := &UserInfo{
		ID:          resp.Sub,
		Username:    resp.Username,
		Email:       resp.Email,
		Roles:       resp.Roles,
		Permissions: resp.Permissions,
		Groups:      resp.Groups,
		Metadata:    make(map[string]string),
	}
	
	// Use name from response if username is not set
	if userInfo.Username == "" && resp.Name != "" {
		userInfo.Username = resp.Name
	}
	
	// Set expiration time
	if resp.Exp > 0 {
		expTime := time.Unix(resp.Exp, 0)
		userInfo.ExpiresAt = &expTime
	}
	
	// Add scope to metadata
	if resp.Scope != "" {
		userInfo.Metadata["scope"] = resp.Scope
	}
	
	// Add client ID to metadata
	if resp.ClientID != "" {
		userInfo.Metadata["client_id"] = resp.ClientID
	}
	
	return userInfo
}

// introspectionToClaims converts introspection response to claims map
func (o *OAuth2Authenticator) introspectionToClaims(resp *IntrospectionResponse) map[string]interface{} {
	claims := make(map[string]interface{})
	
	// Standard claims
	claims["active"] = resp.Active
	if resp.Sub != "" {
		claims["sub"] = resp.Sub
	}
	if resp.Scope != "" {
		claims["scope"] = resp.Scope
	}
	if resp.ClientID != "" {
		claims["client_id"] = resp.ClientID
	}
	if resp.Username != "" {
		claims["username"] = resp.Username
	}
	if resp.TokenType != "" {
		claims["token_type"] = resp.TokenType
	}
	if resp.Exp > 0 {
		claims["exp"] = resp.Exp
	}
	if resp.Iat > 0 {
		claims["iat"] = resp.Iat
	}
	if resp.Nbf > 0 {
		claims["nbf"] = resp.Nbf
	}
	if resp.Aud != "" {
		claims["aud"] = resp.Aud
	}
	if resp.Iss != "" {
		claims["iss"] = resp.Iss
	}
	if resp.Jti != "" {
		claims["jti"] = resp.Jti
	}
	
	// Extension claims
	if resp.Email != "" {
		claims["email"] = resp.Email
	}
	if resp.Name != "" {
		claims["name"] = resp.Name
	}
	if len(resp.Roles) > 0 {
		claims["roles"] = resp.Roles
	}
	if len(resp.Permissions) > 0 {
		claims["permissions"] = resp.Permissions
	}
	if len(resp.Groups) > 0 {
		claims["groups"] = resp.Groups
	}
	
	// Add extra claims
	for k, v := range resp.Extra {
		claims[k] = v
	}
	
	return claims
}

// GetName returns the name of the authenticator
func (o *OAuth2Authenticator) GetName() string {
	return "oauth2"
}
