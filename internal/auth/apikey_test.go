package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
)

func TestAPIKeyAuthenticator_Authenticate(t *testing.T) {
	// Create test configuration
	cfg := &config.APIKeyConfig{
		Header: "X-API-Key",
		Query:  "api_key",
		Keys:   []string{"test-key-1", "test-key-2"},
	}

	// Create authenticator
	auth := NewAPIKeyAuthenticator(cfg)

	tests := []struct {
		name           string
		setupRequest   func() *http.Request
		expectedAuth   bool
		expectedError  string
		expectedStatus int
	}{
		{
			name: "Valid API key in header",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-API-Key", "test-key-1")
				return req
			},
			expectedAuth: true,
		},
		{
			name: "Valid API key in query parameter",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test?api_key=test-key-2", nil)
				return req
			},
			expectedAuth: true,
		},
		{
			name: "Missing API key",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				return req
			},
			expectedAuth:   false,
			expectedError:  "API key not provided",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Invalid API key",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-API-Key", "invalid-key")
				return req
			},
			expectedAuth:   false,
			expectedError:  "Invalid API key",
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
				if result.Error != tt.expectedError {
					t.Errorf("Expected error=%q, got %q", tt.expectedError, result.Error)
				}
				if result.StatusCode != tt.expectedStatus {
					t.Errorf("Expected status=%d, got %d", tt.expectedStatus, result.StatusCode)
				}
			} else {
				if result.UserInfo == nil {
					t.Error("Expected UserInfo to be set for authenticated request")
				}
				if result.Consumer == nil {
					t.Error("Expected Consumer to be set for authenticated request")
				}
			}
		})
	}
}

func TestAPIKeyAuthenticator_GetName(t *testing.T) {
	cfg := &config.APIKeyConfig{
		Header: "X-API-Key",
		Keys:   []string{"test-key"},
	}

	auth := NewAPIKeyAuthenticator(cfg)
	name := auth.GetName()

	if name != "api_key" {
		t.Errorf("Expected name='api_key', got %q", name)
	}
}

func TestConsumerManager_AddConsumer(t *testing.T) {
	cm := NewConsumerManager()

	consumer := &Consumer{
		ID:        "test-consumer",
		Name:      "Test Consumer",
		APIKey:    "test-key",
		HashedKey: cm.hashAPIKey("test-key"),
		Enabled:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := cm.AddConsumer(consumer)
	if err != nil {
		t.Fatalf("Failed to add consumer: %v", err)
	}

	// Test retrieving the consumer
	retrieved, err := cm.GetConsumerByAPIKey("test-key")
	if err != nil {
		t.Fatalf("Failed to get consumer: %v", err)
	}

	if retrieved.ID != consumer.ID {
		t.Errorf("Expected consumer ID=%q, got %q", consumer.ID, retrieved.ID)
	}
}

func TestConsumerManager_GetConsumerByAPIKey(t *testing.T) {
	cm := NewConsumerManager()

	// Test with non-existent key
	_, err := cm.GetConsumerByAPIKey("non-existent-key")
	if err == nil {
		t.Error("Expected error for non-existent key")
	}

	// Add a consumer
	consumer := &Consumer{
		ID:        "test-consumer",
		Name:      "Test Consumer",
		APIKey:    "test-key",
		HashedKey: cm.hashAPIKey("test-key"),
		Enabled:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	cm.AddConsumer(consumer)

	// Test retrieving the consumer
	retrieved, err := cm.GetConsumerByAPIKey("test-key")
	if err != nil {
		t.Fatalf("Failed to get consumer: %v", err)
	}

	if retrieved.ID != consumer.ID {
		t.Errorf("Expected consumer ID=%q, got %q", consumer.ID, retrieved.ID)
	}
}

func TestConsumerManager_UpdateConsumerStats(t *testing.T) {
	cm := NewConsumerManager()

	consumer := &Consumer{
		ID:           "test-consumer",
		Name:         "Test Consumer",
		APIKey:       "test-key",
		HashedKey:    cm.hashAPIKey("test-key"),
		Enabled:      true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		RequestCount: 0,
	}

	cm.AddConsumer(consumer)

	// Update stats
	cm.UpdateConsumerStats("test-consumer")

	// Retrieve and check stats
	retrieved, err := cm.GetConsumer("test-consumer")
	if err != nil {
		t.Fatalf("Failed to get consumer: %v", err)
	}

	if retrieved.RequestCount != 1 {
		t.Errorf("Expected request count=1, got %d", retrieved.RequestCount)
	}

	if retrieved.LastUsedAt == nil {
		t.Error("Expected LastUsedAt to be set")
	}
}

func TestConsumerManager_ListConsumers(t *testing.T) {
	cm := NewConsumerManager()

	// Initially empty
	consumers := cm.ListConsumers()
	if len(consumers) != 0 {
		t.Errorf("Expected 0 consumers, got %d", len(consumers))
	}

	// Add consumers
	consumer1 := &Consumer{
		ID:        "consumer-1",
		Name:      "Consumer 1",
		APIKey:    "key-1",
		HashedKey: cm.hashAPIKey("key-1"),
		Enabled:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	consumer2 := &Consumer{
		ID:        "consumer-2",
		Name:      "Consumer 2",
		APIKey:    "key-2",
		HashedKey: cm.hashAPIKey("key-2"),
		Enabled:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	cm.AddConsumer(consumer1)
	cm.AddConsumer(consumer2)

	// List consumers
	consumers = cm.ListConsumers()
	if len(consumers) != 2 {
		t.Errorf("Expected 2 consumers, got %d", len(consumers))
	}
}

func TestConsumerManager_RemoveConsumer(t *testing.T) {
	cm := NewConsumerManager()

	consumer := &Consumer{
		ID:        "test-consumer",
		Name:      "Test Consumer",
		APIKey:    "test-key",
		HashedKey: cm.hashAPIKey("test-key"),
		Enabled:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	cm.AddConsumer(consumer)

	// Remove consumer
	err := cm.RemoveConsumer("test-consumer")
	if err != nil {
		t.Fatalf("Failed to remove consumer: %v", err)
	}

	// Verify removal
	_, err = cm.GetConsumer("test-consumer")
	if err == nil {
		t.Error("Expected error when getting removed consumer")
	}

	// Test removing non-existent consumer
	err = cm.RemoveConsumer("non-existent")
	if err == nil {
		t.Error("Expected error when removing non-existent consumer")
	}
}
