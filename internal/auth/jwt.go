package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/songzhibin97/stargate/internal/config"
)

// JWTAuthenticator handles JWT authentication
type JWTAuthenticator struct {
	config    *config.JWTConfig
	publicKey interface{}
	jwksCache *JWKSCache
	mu        sync.RWMutex
}

// JWKSCache caches JWKS (JSON Web Key Set) data
type JWKSCache struct {
	keys      map[string]interface{} // key ID -> public key
	lastFetch time.Time
	ttl       time.Duration
	jwksURL   string
	mu        sync.RWMutex
}

// JWK represents a JSON Web Key
type JWK struct {
	Kty string `json:"kty"` // Key Type
	Use string `json:"use"` // Public Key Use
	Kid string `json:"kid"` // Key ID
	N   string `json:"n"`   // Modulus (for RSA)
	E   string `json:"e"`   // Exponent (for RSA)
	X5c []string `json:"x5c"` // X.509 Certificate Chain
}

// JWKS represents a JSON Web Key Set
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWTClaims represents JWT claims with standard and custom fields
type JWTClaims struct {
	jwt.RegisteredClaims
	
	// Custom claims
	Scope       string                 `json:"scope,omitempty"`
	Permissions []string               `json:"permissions,omitempty"`
	Roles       []string               `json:"roles,omitempty"`
	Groups      []string               `json:"groups,omitempty"`
	Email       string                 `json:"email,omitempty"`
	Name        string                 `json:"name,omitempty"`
	Username    string                 `json:"username,omitempty"`
	Custom      map[string]interface{} `json:"-"`
}

// NewJWTAuthenticator creates a new JWT authenticator
func NewJWTAuthenticator(config *config.JWTConfig) (*JWTAuthenticator, error) {
	auth := &JWTAuthenticator{
		config: config,
	}
	
	// Initialize public key or JWKS cache
	if err := auth.initializeKeys(); err != nil {
		return nil, fmt.Errorf("failed to initialize JWT keys: %w", err)
	}
	
	return auth, nil
}

// initializeKeys initializes public keys for JWT verification
func (j *JWTAuthenticator) initializeKeys() error {
	// If JWKS URL is provided, initialize JWKS cache
	if j.config.JWKSURL != "" {
		j.jwksCache = &JWKSCache{
			keys:    make(map[string]interface{}),
			ttl:     5 * time.Minute, // Default TTL
			jwksURL: j.config.JWKSURL,
		}
		
		// Fetch initial keys
		return j.jwksCache.refresh()
	}
	
	// If secret is provided, use it as HMAC key
	if j.config.Secret != "" {
		j.publicKey = []byte(j.config.Secret)
		return nil
	}
	
	// If public key is provided, parse it
	if j.config.PublicKey != "" {
		key, err := j.parsePublicKey(j.config.PublicKey)
		if err != nil {
			return fmt.Errorf("failed to parse public key: %w", err)
		}
		j.publicKey = key
		return nil
	}
	
	return fmt.Errorf("no JWT verification key configured")
}

// parsePublicKey parses a PEM-encoded public key
func (j *JWTAuthenticator) parsePublicKey(keyData string) (interface{}, error) {
	block, _ := pem.Decode([]byte(keyData))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	
	switch block.Type {
	case "PUBLIC KEY":
		return x509.ParsePKIXPublicKey(block.Bytes)
	case "RSA PUBLIC KEY":
		return x509.ParsePKCS1PublicKey(block.Bytes)
	case "CERTIFICATE":
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		return cert.PublicKey, nil
	default:
		return nil, fmt.Errorf("unsupported key type: %s", block.Type)
	}
}

// Authenticate authenticates a request using JWT
func (j *JWTAuthenticator) Authenticate(r *http.Request) (*AuthResult, error) {
	// Extract JWT token from Authorization header
	token := j.extractToken(r)
	if token == "" {
		return &AuthResult{
			Authenticated: false,
			Error:         "JWT token not provided",
			StatusCode:    http.StatusUnauthorized,
		}, nil
	}
	
	// Parse and validate the token
	claims, err := j.validateToken(token)
	if err != nil {
		return &AuthResult{
			Authenticated: false,
			Error:         fmt.Sprintf("Invalid JWT token: %v", err),
			StatusCode:    http.StatusUnauthorized,
		}, nil
	}
	
	// Create user info from claims
	userInfo := j.createUserInfoFromClaims(claims)
	
	// Convert claims to map for context
	claimsMap := j.claimsToMap(claims)
	
	return &AuthResult{
		Authenticated: true,
		UserInfo:      userInfo,
		Claims:        claimsMap,
	}, nil
}

// extractToken extracts JWT token from Authorization header
func (j *JWTAuthenticator) extractToken(r *http.Request) string {
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

// validateToken validates a JWT token
func (j *JWTAuthenticator) validateToken(tokenString string) (*JWTClaims, error) {
	// Parse token with custom claims
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, j.getKeyFunc())
	if err != nil {
		return nil, err
	}
	
	// Check if token is valid
	if !token.Valid {
		return nil, fmt.Errorf("token is not valid")
	}
	
	// Extract claims
	claims, ok := token.Claims.(*JWTClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims type")
	}
	
	// Validate standard claims
	if err := j.validateStandardClaims(claims); err != nil {
		return nil, err
	}
	
	return claims, nil
}

// getKeyFunc returns a function to get the verification key
func (j *JWTAuthenticator) getKeyFunc() jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {
		// Check signing method
		if j.config.Algorithm != "" {
			expectedMethod := jwt.GetSigningMethod(j.config.Algorithm)
			if expectedMethod == nil {
				return nil, fmt.Errorf("unsupported signing method: %s", j.config.Algorithm)
			}
			
			if token.Method != expectedMethod {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
		}
		
		// If using JWKS, get key by kid
		if j.jwksCache != nil {
			kid, ok := token.Header["kid"].(string)
			if !ok {
				return nil, fmt.Errorf("token missing kid header")
			}
			
			key, err := j.jwksCache.getKey(kid)
			if err != nil {
				return nil, err
			}
			
			return key, nil
		}
		
		// Use configured public key
		return j.publicKey, nil
	}
}

// validateStandardClaims validates standard JWT claims
func (j *JWTAuthenticator) validateStandardClaims(claims *JWTClaims) error {
	now := time.Now()
	
	// Validate expiration time
	if claims.ExpiresAt != nil && claims.ExpiresAt.Before(now) {
		return fmt.Errorf("token has expired")
	}
	
	// Validate not before time
	if claims.NotBefore != nil && claims.NotBefore.After(now) {
		return fmt.Errorf("token not valid yet")
	}
	
	// Validate issued at time (with some leeway)
	if claims.IssuedAt != nil && claims.IssuedAt.After(now.Add(5*time.Minute)) {
		return fmt.Errorf("token issued in the future")
	}
	
	// Validate issuer if configured
	if j.config.Issuer != "" && claims.Issuer != j.config.Issuer {
		return fmt.Errorf("invalid issuer: expected %s, got %s", j.config.Issuer, claims.Issuer)
	}
	
	// Validate audience if configured
	if j.config.Audience != "" {
		if len(claims.Audience) == 0 {
			return fmt.Errorf("token missing audience")
		}
		
		validAudience := false
		for _, aud := range claims.Audience {
			if aud == j.config.Audience {
				validAudience = true
				break
			}
		}
		
		if !validAudience {
			return fmt.Errorf("invalid audience")
		}
	}
	
	return nil
}

// createUserInfoFromClaims creates UserInfo from JWT claims
func (j *JWTAuthenticator) createUserInfoFromClaims(claims *JWTClaims) *UserInfo {
	userInfo := &UserInfo{
		ID:          claims.Subject,
		Username:    claims.Username,
		Email:       claims.Email,
		Roles:       claims.Roles,
		Permissions: claims.Permissions,
		Groups:      claims.Groups,
		Metadata:    make(map[string]string),
	}
	
	// Use name from claims if username is not set
	if userInfo.Username == "" && claims.Name != "" {
		userInfo.Username = claims.Name
	}
	
	// Set expiration time
	if claims.ExpiresAt != nil {
		userInfo.ExpiresAt = &claims.ExpiresAt.Time
	}
	
	return userInfo
}

// claimsToMap converts JWT claims to a map
func (j *JWTAuthenticator) claimsToMap(claims *JWTClaims) map[string]interface{} {
	result := make(map[string]interface{})
	
	// Standard claims
	if claims.Subject != "" {
		result["sub"] = claims.Subject
	}
	if claims.Issuer != "" {
		result["iss"] = claims.Issuer
	}
	if len(claims.Audience) > 0 {
		result["aud"] = claims.Audience
	}
	if claims.ExpiresAt != nil {
		result["exp"] = claims.ExpiresAt.Unix()
	}
	if claims.NotBefore != nil {
		result["nbf"] = claims.NotBefore.Unix()
	}
	if claims.IssuedAt != nil {
		result["iat"] = claims.IssuedAt.Unix()
	}
	if claims.ID != "" {
		result["jti"] = claims.ID
	}
	
	// Custom claims
	if claims.Scope != "" {
		result["scope"] = claims.Scope
	}
	if len(claims.Permissions) > 0 {
		result["permissions"] = claims.Permissions
	}
	if len(claims.Roles) > 0 {
		result["roles"] = claims.Roles
	}
	if len(claims.Groups) > 0 {
		result["groups"] = claims.Groups
	}
	if claims.Email != "" {
		result["email"] = claims.Email
	}
	if claims.Name != "" {
		result["name"] = claims.Name
	}
	if claims.Username != "" {
		result["username"] = claims.Username
	}
	
	// Add custom claims
	for k, v := range claims.Custom {
		result[k] = v
	}
	
	return result
}

// GetName returns the name of the authenticator
func (j *JWTAuthenticator) GetName() string {
	return "jwt"
}

// refresh fetches and caches JWKS from the configured URL
func (c *JWKSCache) refresh() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if cache is still valid
	if time.Since(c.lastFetch) < c.ttl {
		return nil
	}

	// Fetch JWKS from URL
	resp, err := http.Get(c.jwksURL)
	if err != nil {
		return fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	// Parse JWKS response
	var jwks JWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("failed to decode JWKS: %w", err)
	}

	// Convert JWKs to public keys
	newKeys := make(map[string]interface{})
	for _, jwk := range jwks.Keys {
		key, err := c.jwkToPublicKey(&jwk)
		if err != nil {
			// Log error but continue with other keys
			continue
		}
		newKeys[jwk.Kid] = key
	}

	// Update cache
	c.keys = newKeys
	c.lastFetch = time.Now()

	return nil
}

// getKey gets a public key by key ID
func (c *JWKSCache) getKey(kid string) (interface{}, error) {
	// Try to refresh cache if needed
	if err := c.refresh(); err != nil {
		// Log error but try to use cached keys
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	key, exists := c.keys[kid]
	if !exists {
		return nil, fmt.Errorf("key with ID %s not found", kid)
	}

	return key, nil
}

// jwkToPublicKey converts a JWK to a public key
func (c *JWKSCache) jwkToPublicKey(jwk *JWK) (interface{}, error) {
	switch jwk.Kty {
	case "RSA":
		return c.rsaJWKToPublicKey(jwk)
	default:
		return nil, fmt.Errorf("unsupported key type: %s", jwk.Kty)
	}
}

// rsaJWKToPublicKey converts an RSA JWK to an RSA public key
func (c *JWKSCache) rsaJWKToPublicKey(jwk *JWK) (*rsa.PublicKey, error) {
	// If x5c (certificate chain) is available, use the first certificate
	if len(jwk.X5c) > 0 {
		certData := jwk.X5c[0]

		// Decode base64 certificate
		certBytes, err := base64.StdEncoding.DecodeString(certData)
		if err != nil {
			return nil, fmt.Errorf("failed to decode certificate: %w", err)
		}

		// Parse certificate
		cert, err := x509.ParseCertificate(certBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse certificate: %w", err)
		}

		// Extract RSA public key
		rsaKey, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("certificate does not contain RSA public key")
		}

		return rsaKey, nil
	}

	// Use n and e parameters
	if jwk.N == "" || jwk.E == "" {
		return nil, fmt.Errorf("RSA JWK missing n or e parameter")
	}

	// Decode n (modulus) - JWT uses base64url encoding
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("failed to decode n parameter: %w", err)
	}

	// Decode e (exponent) - JWT uses base64url encoding
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("failed to decode e parameter: %w", err)
	}

	// Convert bytes to big integers
	publicKey := &rsa.PublicKey{}
	publicKey.N = new(big.Int).SetBytes(nBytes)

	// Convert exponent bytes to int
	var e int
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}
	publicKey.E = e

	return publicKey, nil
}
