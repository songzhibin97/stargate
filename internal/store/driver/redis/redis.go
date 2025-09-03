package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/songzhibin97/stargate/pkg/store"
)

// RedisStore implements the store.AtomicStore interface using Redis
type RedisStore struct {
	client    *redis.Client
	keyPrefix string
	config    *store.Config
}

// New creates a new Redis store instance
func New(config *store.Config) (store.AtomicStore, error) {
	if config == nil {
		config = store.DefaultConfig()
	}

	// Validate required configuration
	if config.Address == "" {
		return nil, fmt.Errorf("redis address is required")
	}

	// Create Redis client options
	opts := &redis.Options{
		Addr:     config.Address,
		Password: config.Password,
		DB:       config.Database,
	}

	// Set timeout if specified
	if config.Timeout > 0 {
		opts.DialTimeout = config.Timeout
		opts.ReadTimeout = config.Timeout
		opts.WriteTimeout = config.Timeout
	}

	// Create Redis client
	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisStore{
		client:    client,
		keyPrefix: config.KeyPrefix,
		config:    config,
	}, nil
}

// getKey returns the full key with prefix
func (rs *RedisStore) getKey(key string) string {
	if rs.keyPrefix == "" {
		return key
	}
	return rs.keyPrefix + ":" + key
}

// IncrBy atomically increments the value of a key by the given amount
func (rs *RedisStore) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	fullKey := rs.getKey(key)
	
	result, err := rs.client.IncrBy(ctx, fullKey, value).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment key %s: %w", key, err)
	}
	
	return result, nil
}

// Set stores a value by key with optional TTL
func (rs *RedisStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	fullKey := rs.getKey(key)
	
	var err error
	if ttl > 0 {
		err = rs.client.Set(ctx, fullKey, value, ttl).Err()
	} else {
		err = rs.client.Set(ctx, fullKey, value, 0).Err()
	}
	
	if err != nil {
		return fmt.Errorf("failed to set key %s: %w", key, err)
	}
	
	return nil
}

// Get retrieves a value by key
func (rs *RedisStore) Get(ctx context.Context, key string) ([]byte, error) {
	fullKey := rs.getKey(key)
	
	result, err := rs.client.Get(ctx, fullKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Key doesn't exist
		}
		return nil, fmt.Errorf("failed to get key %s: %w", key, err)
	}
	
	return []byte(result), nil
}

// Delete removes a key from storage
func (rs *RedisStore) Delete(ctx context.Context, key string) error {
	fullKey := rs.getKey(key)
	
	err := rs.client.Del(ctx, fullKey).Err()
	if err != nil {
		return fmt.Errorf("failed to delete key %s: %w", key, err)
	}
	
	return nil
}

// Exists checks if a key exists in storage
func (rs *RedisStore) Exists(ctx context.Context, key string) (bool, error) {
	fullKey := rs.getKey(key)
	
	result, err := rs.client.Exists(ctx, fullKey).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check existence of key %s: %w", key, err)
	}
	
	return result > 0, nil
}

// TTL returns the remaining time to live for a key
func (rs *RedisStore) TTL(ctx context.Context, key string) (time.Duration, error) {
	fullKey := rs.getKey(key)

	result, err := rs.client.TTL(ctx, fullKey).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get TTL for key %s: %w", key, err)
	}

	// Redis returns -2 if key doesn't exist, -1 if key has no expiration
	// Convert to seconds for consistency with our interface
	if result < 0 {
		if result == -2*time.Nanosecond || result == -2*time.Second {
			return -2 * time.Second, nil // Key doesn't exist
		} else if result == -1*time.Nanosecond || result == -1*time.Second {
			return -1 * time.Second, nil // Key has no expiration
		}
	}

	return result, nil
}

// Close closes the store connection and releases resources
func (rs *RedisStore) Close() error {
	if rs.client != nil {
		return rs.client.Close()
	}
	return nil
}

// Health returns the health status of the store
func (rs *RedisStore) Health(ctx context.Context) store.HealthStatus {
	health := store.HealthStatus{
		Status:    "healthy",
		Message:   "Redis store is operational",
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"type":     "redis",
			"address":  rs.config.Address,
			"database": rs.config.Database,
		},
	}
	
	// Test connection
	if err := rs.client.Ping(ctx).Err(); err != nil {
		health.Status = "unhealthy"
		health.Message = fmt.Sprintf("Redis connection failed: %v", err)
		health.Details["error"] = err.Error()
		return health
	}
	
	// Get additional Redis info
	if info, err := rs.client.Info(ctx, "server").Result(); err == nil {
		health.Details["server_info"] = parseRedisInfo(info)
	}
	
	// Get memory usage info
	if info, err := rs.client.Info(ctx, "memory").Result(); err == nil {
		health.Details["memory_info"] = parseRedisInfo(info)
	}
	
	return health
}

// parseRedisInfo parses Redis INFO command output into a map
func parseRedisInfo(info string) map[string]string {
	result := make(map[string]string)
	lines := []rune(info)
	
	var currentLine []rune
	for _, char := range lines {
		if char == '\n' || char == '\r' {
			if len(currentLine) > 0 {
				line := string(currentLine)
				if len(line) > 0 && line[0] != '#' {
					// Parse key:value pairs
					for i, c := range line {
						if c == ':' {
							key := line[:i]
							value := line[i+1:]
							result[key] = value
							break
						}
					}
				}
				currentLine = currentLine[:0]
			}
		} else {
			currentLine = append(currentLine, char)
		}
	}
	
	// Handle last line if it doesn't end with newline
	if len(currentLine) > 0 {
		line := string(currentLine)
		if len(line) > 0 && line[0] != '#' {
			for i, c := range line {
				if c == ':' {
					key := line[:i]
					value := line[i+1:]
					result[key] = value
					break
				}
			}
		}
	}
	
	return result
}

// parseInt64 safely parses a string to int64
func parseInt64(s string) int64 {
	if val, err := strconv.ParseInt(s, 10, 64); err == nil {
		return val
	}
	return 0
}
