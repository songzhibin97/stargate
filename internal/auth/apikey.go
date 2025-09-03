package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
)

// APIKeyAuthenticator handles API key authentication
type APIKeyAuthenticator struct {
	config    *config.APIKeyConfig
	consumers *ConsumerManager
	mu        sync.RWMutex
}

// Consumer represents an API key consumer
type Consumer struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	APIKey      string            `json:"api_key"`
	HashedKey   string            `json:"hashed_key"`
	Enabled     bool              `json:"enabled"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	LastUsedAt  *time.Time        `json:"last_used_at,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	
	// Rate limiting and access control
	RateLimit   *RateLimitConfig  `json:"rate_limit,omitempty"`
	IPWhitelist []string          `json:"ip_whitelist,omitempty"`
	
	// Statistics
	RequestCount int64 `json:"request_count"`
}

// RateLimitConfig represents rate limiting configuration for a consumer
type RateLimitConfig struct {
	RequestsPerSecond int           `json:"requests_per_second"`
	BurstSize         int           `json:"burst_size"`
	WindowSize        time.Duration `json:"window_size"`
}

// ConsumerManager manages API key consumers
type ConsumerManager struct {
	consumers map[string]*Consumer // key: hashed API key
	mu        sync.RWMutex
}

// NewAPIKeyAuthenticator creates a new API key authenticator
func NewAPIKeyAuthenticator(config *config.APIKeyConfig) *APIKeyAuthenticator {
	auth := &APIKeyAuthenticator{
		config:    config,
		consumers: NewConsumerManager(),
	}
	
	// Initialize with configured keys if any
	auth.initializeConsumers()
	
	return auth
}

// NewConsumerManager creates a new consumer manager
func NewConsumerManager() *ConsumerManager {
	return &ConsumerManager{
		consumers: make(map[string]*Consumer),
	}
}

// initializeConsumers initializes consumers from configuration
func (a *APIKeyAuthenticator) initializeConsumers() {
	if a.config == nil || len(a.config.Keys) == 0 {
		return
	}
	
	for i, key := range a.config.Keys {
		if key == "" {
			continue
		}
		
		consumer := &Consumer{
			ID:        fmt.Sprintf("config-consumer-%d", i),
			Name:      fmt.Sprintf("Config Consumer %d", i+1),
			APIKey:    key,
			HashedKey: a.hashAPIKey(key),
			Enabled:   true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Metadata:  make(map[string]string),
		}
		
		a.consumers.AddConsumer(consumer)
	}
}

// Authenticate authenticates a request using API key
func (a *APIKeyAuthenticator) Authenticate(r *http.Request) (*AuthResult, error) {
	// Extract API key from request
	apiKey := a.extractAPIKey(r)
	if apiKey == "" {
		return &AuthResult{
			Authenticated: false,
			Error:         "API key not provided",
			StatusCode:    http.StatusUnauthorized,
		}, nil
	}
	
	// Find consumer by API key
	consumer, err := a.consumers.GetConsumerByAPIKey(apiKey)
	if err != nil {
		return &AuthResult{
			Authenticated: false,
			Error:         "Invalid API key",
			StatusCode:    http.StatusUnauthorized,
		}, nil
	}
	
	// Check if consumer is enabled
	if !consumer.Enabled {
		return &AuthResult{
			Authenticated: false,
			Error:         "API key is disabled",
			StatusCode:    http.StatusForbidden,
		}, nil
	}
	
	// Check IP whitelist if configured
	if len(consumer.IPWhitelist) > 0 {
		clientIP := a.getClientIP(r)
		if !a.isIPWhitelisted(clientIP, consumer.IPWhitelist) {
			return &AuthResult{
				Authenticated: false,
				Error:         "IP address not whitelisted",
				StatusCode:    http.StatusForbidden,
			}, nil
		}
	}
	
	// Update consumer statistics
	a.consumers.UpdateConsumerStats(consumer.ID)
	
	// Create user info
	userInfo := &UserInfo{
		ID:       consumer.ID,
		Username: consumer.Name,
		Metadata: consumer.Metadata,
	}
	
	return &AuthResult{
		Authenticated: true,
		UserInfo:      userInfo,
		Consumer:      consumer,
	}, nil
}

// GetName returns the name of the authenticator
func (a *APIKeyAuthenticator) GetName() string {
	return "api_key"
}

// extractAPIKey extracts API key from request headers or query parameters
func (a *APIKeyAuthenticator) extractAPIKey(r *http.Request) string {
	// Try header first
	if a.config.Header != "" {
		if key := r.Header.Get(a.config.Header); key != "" {
			return key
		}
	}
	
	// Try query parameter
	if a.config.Query != "" {
		if key := r.URL.Query().Get(a.config.Query); key != "" {
			return key
		}
	}
	
	return ""
}

// hashAPIKey creates a hash of the API key for secure storage
func (a *APIKeyAuthenticator) hashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(hash[:])
}

// getClientIP extracts client IP from request
func (a *APIKeyAuthenticator) getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}
	
	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	
	// Use remote address
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	
	return ip
}

// isIPWhitelisted checks if IP is in whitelist
func (a *APIKeyAuthenticator) isIPWhitelisted(ip string, whitelist []string) bool {
	for _, whitelistedIP := range whitelist {
		if ip == whitelistedIP {
			return true
		}
		// TODO: Add CIDR support
	}
	return false
}

// AddConsumer adds a new consumer
func (cm *ConsumerManager) AddConsumer(consumer *Consumer) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	if consumer.HashedKey == "" {
		return fmt.Errorf("consumer hashed key cannot be empty")
	}
	
	cm.consumers[consumer.HashedKey] = consumer
	return nil
}

// GetConsumerByAPIKey finds a consumer by API key
func (cm *ConsumerManager) GetConsumerByAPIKey(apiKey string) (*Consumer, error) {
	hashedKey := cm.hashAPIKey(apiKey)
	
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	consumer, exists := cm.consumers[hashedKey]
	if !exists {
		return nil, fmt.Errorf("consumer not found")
	}
	
	return consumer, nil
}

// hashAPIKey creates a hash of the API key
func (cm *ConsumerManager) hashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(hash[:])
}

// UpdateConsumerStats updates consumer usage statistics
func (cm *ConsumerManager) UpdateConsumerStats(consumerID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	for _, consumer := range cm.consumers {
		if consumer.ID == consumerID {
			consumer.RequestCount++
			now := time.Now()
			consumer.LastUsedAt = &now
			consumer.UpdatedAt = now
			break
		}
	}
}

// GetConsumer gets a consumer by ID
func (cm *ConsumerManager) GetConsumer(id string) (*Consumer, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	for _, consumer := range cm.consumers {
		if consumer.ID == id {
			return consumer, nil
		}
	}
	
	return nil, fmt.Errorf("consumer not found")
}

// ListConsumers returns all consumers
func (cm *ConsumerManager) ListConsumers() []*Consumer {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	consumers := make([]*Consumer, 0, len(cm.consumers))
	for _, consumer := range cm.consumers {
		consumers = append(consumers, consumer)
	}
	
	return consumers
}

// RemoveConsumer removes a consumer
func (cm *ConsumerManager) RemoveConsumer(id string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	for hashedKey, consumer := range cm.consumers {
		if consumer.ID == id {
			delete(cm.consumers, hashedKey)
			return nil
		}
	}
	
	return fmt.Errorf("consumer not found")
}
