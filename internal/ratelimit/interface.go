package ratelimit

import (
	"fmt"
	"net/http"
	"time"

	"github.com/songzhibin97/stargate/pkg/store"
	"github.com/songzhibin97/stargate/internal/store/driver/memory"
	"github.com/songzhibin97/stargate/internal/store/driver/redis"
)

// RateLimiter defines the interface for rate limiting implementations
type RateLimiter interface {
	// IsAllowed checks if a request from the given identifier is allowed
	IsAllowed(identifier string) bool
	
	// GetQuota returns the current quota information for an identifier
	GetQuota(identifier string) *QuotaInfo
	
	// GetStats returns statistics about the rate limiter
	GetStats() *RateLimiterStats
	
	// Stop stops the rate limiter and cleans up resources
	Stop()
}

// RateLimitResult represents the result of a rate limit check
type RateLimitResult struct {
	Allowed   bool          // whether the request is allowed
	Quota     *QuotaInfo    // quota information
	RetryAfter time.Duration // how long to wait before retrying (if not allowed)
}

// RateLimitStrategy defines different rate limiting strategies
type RateLimitStrategy string

const (
	// StrategyFixedWindow uses fixed time windows
	StrategyFixedWindow RateLimitStrategy = "fixed_window"
	
	// StrategySlidingWindow uses sliding time windows
	StrategySlidingWindow RateLimitStrategy = "sliding_window"
	
	// StrategyTokenBucket uses token bucket algorithm
	StrategyTokenBucket RateLimitStrategy = "token_bucket"
	
	// StrategyLeakyBucket uses leaky bucket algorithm
	StrategyLeakyBucket RateLimitStrategy = "leaky_bucket"
)

// IdentifierStrategy defines how to identify clients
type IdentifierStrategy string

const (
	// IdentifierIP uses client IP address
	IdentifierIP IdentifierStrategy = "ip"
	
	// IdentifierUser uses authenticated user ID
	IdentifierUser IdentifierStrategy = "user"
	
	// IdentifierAPIKey uses API key
	IdentifierAPIKey IdentifierStrategy = "api_key"
	
	// IdentifierCombined uses multiple identifiers
	IdentifierCombined IdentifierStrategy = "combined"
)

// Config represents rate limiter configuration
type Config struct {
	// Strategy defines the rate limiting algorithm to use
	Strategy RateLimitStrategy `yaml:"strategy" json:"strategy"`

	// IdentifierStrategy defines how to identify clients
	IdentifierStrategy IdentifierStrategy `yaml:"identifier_strategy" json:"identifier_strategy"`

	// WindowSize is the duration of each time window (for window-based algorithms)
	WindowSize time.Duration `yaml:"window_size" json:"window_size"`

	// MaxRequests is the maximum number of requests allowed per window
	MaxRequests int `yaml:"max_requests" json:"max_requests"`

	// Rate is the rate for token/leaky bucket algorithms (requests per second)
	Rate float64 `yaml:"rate" json:"rate"`

	// BurstSize is the maximum burst size for token bucket
	BurstSize int `yaml:"burst_size" json:"burst_size"`

	// CleanupInterval defines how often to clean up expired data
	CleanupInterval time.Duration `yaml:"cleanup_interval" json:"cleanup_interval"`

	// Enabled indicates if rate limiting is enabled
	Enabled bool `yaml:"enabled" json:"enabled"`

	// SkipSuccessfulRequests indicates if successful requests should be counted
	SkipSuccessfulRequests bool `yaml:"skip_successful_requests" json:"skip_successful_requests"`

	// SkipFailedRequests indicates if failed requests should be counted
	SkipFailedRequests bool `yaml:"skip_failed_requests" json:"skip_failed_requests"`

	// CustomHeaders allows setting custom rate limit headers
	CustomHeaders map[string]string `yaml:"custom_headers" json:"custom_headers"`

	// Storage defines the storage backend type ("memory" or "redis")
	Storage string `yaml:"storage" json:"storage"`

	// Redis configuration fields (for backward compatibility)
	RedisAddress  string `yaml:"redis_address" json:"redis_address"`
	RedisPassword string `yaml:"redis_password" json:"redis_password"`
	RedisDB       int    `yaml:"redis_db" json:"redis_db"`

	// RedisConfig contains Redis-specific configuration (deprecated, use individual fields)
	RedisConfig *RedisConfig `yaml:"redis_config" json:"redis_config"`
}

// RedisConfig represents Redis configuration for rate limiting
type RedisConfig struct {
	Address  string `yaml:"address" json:"address"`
	Password string `yaml:"password" json:"password"`
	DB       int    `yaml:"db" json:"db"`
}

// DefaultConfig returns a default rate limiter configuration
func DefaultConfig() *Config {
	return &Config{
		Strategy:           StrategyFixedWindow,
		IdentifierStrategy: IdentifierIP,
		WindowSize:         time.Minute,
		MaxRequests:        100,
		Rate:               10.0,
		BurstSize:          20,
		CleanupInterval:    5 * time.Minute,
		Enabled:            true,
		SkipSuccessfulRequests: false,
		SkipFailedRequests:     false,
		CustomHeaders:      make(map[string]string),
		Storage:            "memory",
		RedisAddress:       "localhost:6379",
		RedisPassword:      "",
		RedisDB:            0,
	}
}

// Manager manages multiple rate limiters and provides a unified interface
type Manager struct {
	limiters map[string]RateLimiter // key: limiter name, value: rate limiter
	config   *Config
}

// NewManager creates a new rate limiter manager
func NewManager(config *Config) *Manager {
	if config == nil {
		config = DefaultConfig()
	}
	
	return &Manager{
		limiters: make(map[string]RateLimiter),
		config:   config,
	}
}

// CreateLimiter creates a new rate limiter with the given name and config
func (m *Manager) CreateLimiter(name string, config *Config) (RateLimiter, error) {
	if config == nil {
		config = m.config
	}
	
	var limiter RateLimiter
	var err error

	// Check if distributed storage is requested
	if config.Storage == "redis" {
		return m.createDistributedLimiter(name, config)
	}

	// Default to in-memory storage
	switch config.Strategy {
	case StrategyFixedWindow:
		limiter = NewFixedWindowRateLimiter(&FixedWindowConfig{
			WindowSize:      config.WindowSize,
			MaxRequests:     config.MaxRequests,
			CleanupInterval: config.CleanupInterval,
		})
	case StrategyTokenBucket:
		limiter = NewTokenBucketRateLimiter(&TokenBucketConfig{
			Rate:            config.Rate,
			BurstSize:       config.BurstSize,
			CleanupInterval: config.CleanupInterval,
		})
	case StrategySlidingWindow:
		// TODO: Implement sliding window rate limiter
		return nil, ErrUnsupportedStrategy
	case StrategyLeakyBucket:
		// TODO: Implement leaky bucket rate limiter
		return nil, ErrUnsupportedStrategy
	default:
		return nil, ErrUnsupportedStrategy
	}
	
	if err != nil {
		return nil, err
	}
	
	m.limiters[name] = limiter
	return limiter, nil
}

// createDistributedLimiter creates a distributed rate limiter using store.AtomicStore interface
func (m *Manager) createDistributedLimiter(name string, config *Config) (RateLimiter, error) {
	var atomicStore store.AtomicStore
	var err error

	// Create store based on configuration
	storeConfig := &store.Config{
		Type:      config.Storage,
		Address:   config.RedisAddress,
		Database:  config.RedisDB,
		Password:  config.RedisPassword,
		Timeout:   5 * time.Second,
		KeyPrefix: "ratelimit",
	}

	switch config.Storage {
	case "redis":
		atomicStore, err = redis.New(storeConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create Redis store: %w", err)
		}
	case "memory":
		atomicStore, err = memory.New(storeConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create memory store: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", config.Storage)
	}

	// Create distributed config
	distributedConfig := &DistributedConfig{
		Strategy:           config.Strategy,
		IdentifierStrategy: config.IdentifierStrategy,
		WindowSize:         config.WindowSize,
		MaxRequests:        config.MaxRequests,
		Rate:               config.Rate,
		BurstSize:          config.BurstSize,
		KeyPrefix:          "ratelimit:",
	}

	// Create distributed rate limiter
	limiter := NewDistributedRateLimiter(atomicStore, distributedConfig)

	m.limiters[name] = limiter
	return limiter, nil
}

// GetLimiter returns a rate limiter by name
func (m *Manager) GetLimiter(name string) (RateLimiter, bool) {
	limiter, exists := m.limiters[name]
	return limiter, exists
}

// RemoveLimiter removes a rate limiter by name
func (m *Manager) RemoveLimiter(name string) {
	if limiter, exists := m.limiters[name]; exists {
		limiter.Stop()
		delete(m.limiters, name)
	}
}

// CheckRequest checks if a request is allowed using the specified limiter
func (m *Manager) CheckRequest(limiterName string, r *http.Request) *RateLimitResult {
	limiter, exists := m.limiters[limiterName]
	if !exists {
		// If limiter doesn't exist, allow the request
		return &RateLimitResult{
			Allowed: true,
			Quota: &QuotaInfo{
				Limit:     -1, // unlimited
				Remaining: -1, // unlimited
				ResetTime: time.Time{},
			},
		}
	}
	
	identifier := ExtractIdentifier(r, string(m.config.IdentifierStrategy))
	allowed := limiter.IsAllowed(identifier)
	quota := limiter.GetQuota(identifier)
	
	var retryAfter time.Duration
	if !allowed && quota != nil {
		retryAfter = time.Until(quota.ResetTime)
		if retryAfter < 0 {
			retryAfter = 0
		}
	}
	
	return &RateLimitResult{
		Allowed:    allowed,
		Quota:      quota,
		RetryAfter: retryAfter,
	}
}

// Stop stops all rate limiters and cleans up resources
func (m *Manager) Stop() {
	for _, limiter := range m.limiters {
		limiter.Stop()
	}
	m.limiters = make(map[string]RateLimiter)
}

// GetAllStats returns statistics for all rate limiters
func (m *Manager) GetAllStats() map[string]*RateLimiterStats {
	stats := make(map[string]*RateLimiterStats)
	for name, limiter := range m.limiters {
		stats[name] = limiter.GetStats()
	}
	return stats
}

// Health returns the health status of the rate limiter manager
func (m *Manager) Health() map[string]interface{} {
	return map[string]interface{}{
		"enabled":        m.config.Enabled,
		"strategy":       string(m.config.Strategy),
		"identifier":     string(m.config.IdentifierStrategy),
		"limiters_count": len(m.limiters),
		"window_size":    m.config.WindowSize.String(),
		"max_requests":   m.config.MaxRequests,
	}
}

// Errors
var (
	ErrUnsupportedStrategy = fmt.Errorf("unsupported rate limiting strategy")
	ErrLimiterNotFound     = fmt.Errorf("rate limiter not found")
	ErrInvalidConfig       = fmt.Errorf("invalid rate limiter configuration")
)

// Helper function to create a rate limiter based on strategy
func CreateRateLimiter(strategy RateLimitStrategy, config *Config) (RateLimiter, error) {
	switch strategy {
	case StrategyFixedWindow:
		return NewFixedWindowRateLimiter(&FixedWindowConfig{
			WindowSize:      config.WindowSize,
			MaxRequests:     config.MaxRequests,
			CleanupInterval: config.CleanupInterval,
		}), nil
	case StrategyTokenBucket:
		return NewTokenBucketRateLimiter(&TokenBucketConfig{
			Rate:            config.Rate,
			BurstSize:       config.BurstSize,
			CleanupInterval: config.CleanupInterval,
		}), nil
	default:
		return nil, ErrUnsupportedStrategy
	}
}
