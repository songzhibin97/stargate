package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// PasswordHasher handles password hashing and verification
type PasswordHasher struct {
	cost int
}

// NewPasswordHasher creates a new password hasher
func NewPasswordHasher() *PasswordHasher {
	return &PasswordHasher{
		cost: bcrypt.DefaultCost,
	}
}

// HashPassword hashes a password using bcrypt
func (ph *PasswordHasher) HashPassword(password string) (string, error) {
	if password == "" {
		return "", fmt.Errorf("password cannot be empty")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), ph.cost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	return string(hash), nil
}

// VerifyPassword verifies a password against its hash
func (ph *PasswordHasher) VerifyPassword(password, hash string) error {
	if password == "" {
		return fmt.Errorf("password cannot be empty")
	}
	if hash == "" {
		return fmt.Errorf("hash cannot be empty")
	}

	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		return fmt.Errorf("password verification failed: %w", err)
	}

	return nil
}

// JWTManager handles JWT token generation and validation
type JWTManager struct {
	secret    []byte
	algorithm string
	expiresIn time.Duration
	issuer    string
}

// JWTClaims represents JWT claims for portal users
type JWTClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// NewJWTManager creates a new JWT manager
func NewJWTManager(secret, algorithm string, expiresIn time.Duration, issuer string) (*JWTManager, error) {
	if secret == "" {
		return nil, fmt.Errorf("JWT secret cannot be empty")
	}
	if algorithm == "" {
		algorithm = "HS256"
	}
	if expiresIn == 0 {
		expiresIn = 24 * time.Hour
	}
	if issuer == "" {
		issuer = "stargate-portal"
	}

	return &JWTManager{
		secret:    []byte(secret),
		algorithm: algorithm,
		expiresIn: expiresIn,
		issuer:    issuer,
	}, nil
}

// GenerateToken generates a JWT token for a user
func (jm *JWTManager) GenerateToken(userID, email, role string) (string, error) {
	if userID == "" {
		return "", fmt.Errorf("user ID cannot be empty")
	}
	if email == "" {
		return "", fmt.Errorf("email cannot be empty")
	}

	now := time.Now()
	claims := &JWTClaims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    jm.issuer,
			Subject:   userID,
			Audience:  []string{"stargate-portal"},
			ExpiresAt: jwt.NewNumericDate(now.Add(jm.expiresIn)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.GetSigningMethod(jm.algorithm), claims)
	tokenString, err := token.SignedString(jm.secret)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// ValidateToken validates a JWT token and returns the claims
func (jm *JWTManager) ValidateToken(tokenString string) (*JWTClaims, error) {
	if tokenString == "" {
		return nil, fmt.Errorf("token cannot be empty")
	}

	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify the signing method
		if token.Method.Alg() != jm.algorithm {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jm.secret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("token is not valid")
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims type")
	}

	// Additional validation
	if claims.Issuer != jm.issuer {
		return nil, fmt.Errorf("invalid issuer")
	}

	return claims, nil
}

// APIKeyGenerator generates secure API keys
type APIKeyGenerator struct{}

// NewAPIKeyGenerator creates a new API key generator
func NewAPIKeyGenerator() *APIKeyGenerator {
	return &APIKeyGenerator{}
}

// GenerateAPIKey generates a secure API key with the given prefix
func (akg *APIKeyGenerator) GenerateAPIKey(prefix string) (string, error) {
	if prefix == "" {
		prefix = "sk"
	}

	// Generate 32 random bytes
	randomBytes := make([]byte, 32)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Convert to hex string
	randomHex := hex.EncodeToString(randomBytes)

	// Create API key with prefix
	apiKey := fmt.Sprintf("%s_%s", prefix, randomHex)

	return apiKey, nil
}

// HashAPIKey creates a hash of the API key for secure storage
func (akg *APIKeyGenerator) HashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(hash[:])
}

// VerifyAPIKey verifies an API key against its hash
func (akg *APIKeyGenerator) VerifyAPIKey(apiKey, hash string) bool {
	computedHash := akg.HashAPIKey(apiKey)
	return computedHash == hash
}

// UserIDGenerator generates unique user IDs
type UserIDGenerator struct{}

// NewUserIDGenerator creates a new user ID generator
func NewUserIDGenerator() *UserIDGenerator {
	return &UserIDGenerator{}
}

// GenerateUserID generates a unique user ID
func (ug *UserIDGenerator) GenerateUserID() (string, error) {
	// Generate 16 random bytes
	randomBytes := make([]byte, 16)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Convert to hex string and add prefix
	userID := fmt.Sprintf("usr_%s", hex.EncodeToString(randomBytes))

	return userID, nil
}

// ApplicationIDGenerator generates unique application IDs
type ApplicationIDGenerator struct{}

// NewApplicationIDGenerator creates a new application ID generator
func NewApplicationIDGenerator() *ApplicationIDGenerator {
	return &ApplicationIDGenerator{}
}

// GenerateApplicationID generates a unique application ID
func (ag *ApplicationIDGenerator) GenerateApplicationID() (string, error) {
	// Generate 16 random bytes
	randomBytes := make([]byte, 16)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Convert to hex string and add prefix
	appID := fmt.Sprintf("app_%s", hex.EncodeToString(randomBytes))

	return appID, nil
}

// User represents a user in the system
type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const userContextKey contextKey = "user"

// GetUserFromContext retrieves user information from the request context
func GetUserFromContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(userContextKey).(*User)
	return user, ok
}

// SetUserInContext sets user information in the request context
func SetUserInContext(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}
