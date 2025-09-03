package memory

import (
	"context"
	"testing"
	"time"

	"github.com/songzhibin97/stargate/pkg/store"
)

func TestMemoryStore_IncrBy(t *testing.T) {
	ctx := context.Background()
	ms, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create memory store: %v", err)
	}
	defer ms.Close()

	// Test increment on non-existing key
	result, err := ms.IncrBy(ctx, "counter", 5)
	if err != nil {
		t.Errorf("IncrBy failed: %v", err)
	}
	if result != 5 {
		t.Errorf("Expected 5, got %d", result)
	}

	// Test increment on existing key
	result, err = ms.IncrBy(ctx, "counter", 3)
	if err != nil {
		t.Errorf("IncrBy failed: %v", err)
	}
	if result != 8 {
		t.Errorf("Expected 8, got %d", result)
	}

	// Test negative increment
	result, err = ms.IncrBy(ctx, "counter", -2)
	if err != nil {
		t.Errorf("IncrBy failed: %v", err)
	}
	if result != 6 {
		t.Errorf("Expected 6, got %d", result)
	}
}

func TestMemoryStore_SetGet(t *testing.T) {
	ctx := context.Background()
	ms, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create memory store: %v", err)
	}
	defer ms.Close()

	// Test set and get
	key := "test-key"
	value := []byte("test-value")

	err = ms.Set(ctx, key, value, 0)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	result, err := ms.Get(ctx, key)
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}

	if string(result) != string(value) {
		t.Errorf("Expected %s, got %s", string(value), string(result))
	}
}

func TestMemoryStore_TTL(t *testing.T) {
	ctx := context.Background()
	ms, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create memory store: %v", err)
	}
	defer ms.Close()

	key := "ttl-key"
	value := []byte("ttl-value")
	ttl := 100 * time.Millisecond

	// Set with TTL
	err = ms.Set(ctx, key, value, ttl)
	if err != nil {
		t.Errorf("Set with TTL failed: %v", err)
	}

	// Check TTL
	remainingTTL, err := ms.TTL(ctx, key)
	if err != nil {
		t.Errorf("TTL failed: %v", err)
	}

	if remainingTTL <= 0 || remainingTTL > ttl {
		t.Errorf("Expected TTL between 0 and %v, got %v", ttl, remainingTTL)
	}

	// Wait for expiration
	time.Sleep(ttl + 10*time.Millisecond)

	// Key should be expired
	result, err := ms.Get(ctx, key)
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}

	if result != nil {
		t.Errorf("Expected nil (expired), got %s", string(result))
	}

	// TTL should return -2 (key doesn't exist)
	remainingTTL, err = ms.TTL(ctx, key)
	if err != nil {
		t.Errorf("TTL failed: %v", err)
	}

	if remainingTTL != -2*time.Second {
		t.Errorf("Expected -2s (key doesn't exist), got %v", remainingTTL)
	}
}

func TestMemoryStore_Exists(t *testing.T) {
	ctx := context.Background()
	ms, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create memory store: %v", err)
	}
	defer ms.Close()

	key := "exists-key"

	// Key should not exist initially
	exists, err := ms.Exists(ctx, key)
	if err != nil {
		t.Errorf("Exists failed: %v", err)
	}
	if exists {
		t.Errorf("Expected false, got true")
	}

	// Set the key
	err = ms.Set(ctx, key, []byte("value"), 0)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// Key should exist now
	exists, err = ms.Exists(ctx, key)
	if err != nil {
		t.Errorf("Exists failed: %v", err)
	}
	if !exists {
		t.Errorf("Expected true, got false")
	}
}

func TestMemoryStore_Delete(t *testing.T) {
	ctx := context.Background()
	ms, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create memory store: %v", err)
	}
	defer ms.Close()

	key := "delete-key"
	value := []byte("delete-value")

	// Set the key
	err = ms.Set(ctx, key, value, 0)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// Verify it exists
	exists, err := ms.Exists(ctx, key)
	if err != nil {
		t.Errorf("Exists failed: %v", err)
	}
	if !exists {
		t.Errorf("Expected true, got false")
	}

	// Delete the key
	err = ms.Delete(ctx, key)
	if err != nil {
		t.Errorf("Delete failed: %v", err)
	}

	// Verify it no longer exists
	exists, err = ms.Exists(ctx, key)
	if err != nil {
		t.Errorf("Exists failed: %v", err)
	}
	if exists {
		t.Errorf("Expected false, got true")
	}
}

func TestMemoryStore_Health(t *testing.T) {
	ctx := context.Background()
	ms, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create memory store: %v", err)
	}
	defer ms.Close()

	health := ms.Health(ctx)

	if health.Status != "healthy" {
		t.Errorf("Expected healthy status, got %s", health.Status)
	}

	if health.Details["type"] != "memory" {
		t.Errorf("Expected memory type, got %v", health.Details["type"])
	}
}

func TestMemoryStore_KeyPrefix(t *testing.T) {
	ctx := context.Background()
	config := &store.Config{
		KeyPrefix: "test-prefix",
	}

	ms, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create memory store: %v", err)
	}
	defer ms.Close()

	key := "key"
	value := []byte("value")

	// Set with prefix
	err = ms.Set(ctx, key, value, 0)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	// Get should work with the same key
	result, err := ms.Get(ctx, key)
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}

	if string(result) != string(value) {
		t.Errorf("Expected %s, got %s", string(value), string(result))
	}
}

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	ms, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create memory store: %v", err)
	}
	defer ms.Close()

	// Test concurrent increments
	key := "concurrent-counter"
	numGoroutines := 100
	incrementsPerGoroutine := 10

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < incrementsPerGoroutine; j++ {
				_, err := ms.IncrBy(ctx, key, 1)
				if err != nil {
					t.Errorf("IncrBy failed: %v", err)
				}
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Check final value
	result, err := ms.Get(ctx, key)
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}

	expectedValue := numGoroutines * incrementsPerGoroutine
	if string(result) != "1000" {
		t.Errorf("Expected %d, got %s", expectedValue, string(result))
	}
}
