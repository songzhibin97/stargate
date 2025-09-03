package redis

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/songzhibin97/stargate/pkg/store"
)

// getRedisConfig returns a Redis configuration for testing
func getRedisConfig() *store.Config {
	// Check if Redis is available via environment variable
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	return &store.Config{
		Type:      "redis",
		Address:   redisAddr,
		Database:  0,
		Password:  "",
		Timeout:   5 * time.Second,
		KeyPrefix: "test",
	}
}

// skipIfRedisUnavailable skips the test if Redis is not available
func skipIfRedisUnavailable(t *testing.T, rs store.AtomicStore) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	health := rs.Health(ctx)
	if health.Status != "healthy" {
		t.Skipf("Redis is not available: %s", health.Message)
	}
}

func TestRedisStore_IncrBy(t *testing.T) {
	ctx := context.Background()
	config := getRedisConfig()
	
	rs, err := New(config)
	if err != nil {
		t.Skipf("Failed to create Redis store (Redis may not be available): %v", err)
	}
	defer rs.Close()

	skipIfRedisUnavailable(t, rs)

	// Clean up test key
	testKey := "test-counter"
	defer rs.Delete(ctx, testKey)

	// Test increment on non-existing key
	result, err := rs.IncrBy(ctx, testKey, 5)
	if err != nil {
		t.Errorf("IncrBy failed: %v", err)
	}
	if result != 5 {
		t.Errorf("Expected 5, got %d", result)
	}

	// Test increment on existing key
	result, err = rs.IncrBy(ctx, testKey, 3)
	if err != nil {
		t.Errorf("IncrBy failed: %v", err)
	}
	if result != 8 {
		t.Errorf("Expected 8, got %d", result)
	}

	// Test negative increment
	result, err = rs.IncrBy(ctx, testKey, -2)
	if err != nil {
		t.Errorf("IncrBy failed: %v", err)
	}
	if result != 6 {
		t.Errorf("Expected 6, got %d", result)
	}
}

func TestRedisStore_SetGet(t *testing.T) {
	ctx := context.Background()
	config := getRedisConfig()
	
	rs, err := New(config)
	if err != nil {
		t.Skipf("Failed to create Redis store (Redis may not be available): %v", err)
	}
	defer rs.Close()

	skipIfRedisUnavailable(t, rs)

	// Test set and get
	key := "test-key"
	value := []byte("test-value")

	// Clean up test key
	defer rs.Delete(ctx, key)

	err = rs.Set(ctx, key, value, 0)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	result, err := rs.Get(ctx, key)
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}

	if string(result) != string(value) {
		t.Errorf("Expected %s, got %s", string(value), string(result))
	}
}

func TestRedisStore_TTL(t *testing.T) {
	ctx := context.Background()
	config := getRedisConfig()
	
	rs, err := New(config)
	if err != nil {
		t.Skipf("Failed to create Redis store (Redis may not be available): %v", err)
	}
	defer rs.Close()

	skipIfRedisUnavailable(t, rs)

	key := "ttl-key"
	value := []byte("ttl-value")
	ttl := 2 * time.Second

	// Clean up test key
	defer rs.Delete(ctx, key)

	// Set with TTL
	err = rs.Set(ctx, key, value, ttl)
	if err != nil {
		t.Errorf("Set with TTL failed: %v", err)
	}

	// Check TTL
	remainingTTL, err := rs.TTL(ctx, key)
	if err != nil {
		t.Errorf("TTL failed: %v", err)
	}

	if remainingTTL <= 0 || remainingTTL > ttl {
		t.Errorf("Expected TTL between 0 and %v, got %v", ttl, remainingTTL)
	}

	// Wait for expiration
	time.Sleep(ttl + 100*time.Millisecond)

	// Key should be expired
	result, err := rs.Get(ctx, key)
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}

	if result != nil {
		t.Errorf("Expected nil (expired), got %s", string(result))
	}

	// TTL should return -2 (key doesn't exist)
	remainingTTL, err = rs.TTL(ctx, key)
	if err != nil {
		t.Errorf("TTL failed: %v", err)
	}

	if remainingTTL != -2*time.Second {
		t.Errorf("Expected -2s (key doesn't exist), got %v", remainingTTL)
	}
}

func TestRedisStore_Exists(t *testing.T) {
	ctx := context.Background()
	config := getRedisConfig()
	
	rs, err := New(config)
	if err != nil {
		t.Skipf("Failed to create Redis store (Redis may not be available): %v", err)
	}
	defer rs.Close()

	skipIfRedisUnavailable(t, rs)

	key := "exists-key"

	// Clean up test key
	defer rs.Delete(ctx, key)

	// Key should not exist initially
	exists, err := rs.Exists(ctx, key)
	if err != nil {
		t.Errorf("Exists failed: %v", err)
	}
	if exists {
		t.Errorf("Expected false, got true")
	}

	// Set the key
	err = rs.Set(ctx, key, []byte("value"), 0)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// Key should exist now
	exists, err = rs.Exists(ctx, key)
	if err != nil {
		t.Errorf("Exists failed: %v", err)
	}
	if !exists {
		t.Errorf("Expected true, got false")
	}
}

func TestRedisStore_Delete(t *testing.T) {
	ctx := context.Background()
	config := getRedisConfig()
	
	rs, err := New(config)
	if err != nil {
		t.Skipf("Failed to create Redis store (Redis may not be available): %v", err)
	}
	defer rs.Close()

	skipIfRedisUnavailable(t, rs)

	key := "delete-key"
	value := []byte("delete-value")

	// Set the key
	err = rs.Set(ctx, key, value, 0)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// Verify it exists
	exists, err := rs.Exists(ctx, key)
	if err != nil {
		t.Errorf("Exists failed: %v", err)
	}
	if !exists {
		t.Errorf("Expected true, got false")
	}

	// Delete the key
	err = rs.Delete(ctx, key)
	if err != nil {
		t.Errorf("Delete failed: %v", err)
	}

	// Verify it no longer exists
	exists, err = rs.Exists(ctx, key)
	if err != nil {
		t.Errorf("Exists failed: %v", err)
	}
	if exists {
		t.Errorf("Expected false, got true")
	}
}

func TestRedisStore_Health(t *testing.T) {
	ctx := context.Background()
	config := getRedisConfig()
	
	rs, err := New(config)
	if err != nil {
		t.Skipf("Failed to create Redis store (Redis may not be available): %v", err)
	}
	defer rs.Close()

	health := rs.Health(ctx)

	// Health status should be either healthy or unhealthy
	if health.Status != "healthy" && health.Status != "unhealthy" {
		t.Errorf("Expected healthy or unhealthy status, got %s", health.Status)
	}

	if health.Details["type"] != "redis" {
		t.Errorf("Expected redis type, got %v", health.Details["type"])
	}
}

func TestRedisStore_KeyPrefix(t *testing.T) {
	ctx := context.Background()
	config := getRedisConfig()
	config.KeyPrefix = "test-prefix"
	
	rs, err := New(config)
	if err != nil {
		t.Skipf("Failed to create Redis store (Redis may not be available): %v", err)
	}
	defer rs.Close()

	skipIfRedisUnavailable(t, rs)

	key := "key"
	value := []byte("value")

	// Clean up test key
	defer rs.Delete(ctx, key)

	// Set with prefix
	err = rs.Set(ctx, key, value, 0)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// Get should work with the same key
	result, err := rs.Get(ctx, key)
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}

	if string(result) != string(value) {
		t.Errorf("Expected %s, got %s", string(value), string(result))
	}
}
