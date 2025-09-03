package gateway

import (
	"fmt"
	"time"

	"github.com/songzhibin97/stargate/internal/portal/auth"
)

// MockClient is a mock implementation of the gateway client for testing
type MockClient struct {
	consumers map[string]*Consumer
	apiKeys   map[string][]string
	keyGen    *auth.APIKeyGenerator
}

// NewMockClient creates a new mock gateway client
func NewMockClient() *MockClient {
	return &MockClient{
		consumers: make(map[string]*Consumer),
		apiKeys:   make(map[string][]string),
		keyGen:    auth.NewAPIKeyGenerator(),
	}
}

// CreateConsumer creates a mock consumer
func (mc *MockClient) CreateConsumer(consumerID, name string, metadata map[string]string) (*Consumer, error) {
	if _, exists := mc.consumers[consumerID]; exists {
		return nil, fmt.Errorf("consumer with ID %s already exists", consumerID)
	}

	consumer := &Consumer{
		ID:       consumerID,
		Name:     name,
		Enabled:  true,
		Metadata: metadata,
		RateLimit: &RateLimitConfig{
			RequestsPerSecond: 100,
			BurstSize:         200,
			WindowSize:        time.Minute,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	mc.consumers[consumerID] = consumer
	mc.apiKeys[consumerID] = []string{}

	return consumer, nil
}

// DeleteConsumer deletes a mock consumer
func (mc *MockClient) DeleteConsumer(consumerID string) error {
	if _, exists := mc.consumers[consumerID]; !exists {
		return fmt.Errorf("consumer with ID %s not found", consumerID)
	}

	delete(mc.consumers, consumerID)
	delete(mc.apiKeys, consumerID)
	return nil
}

// GenerateAPIKey generates a mock API key for a consumer
func (mc *MockClient) GenerateAPIKey(consumerID string) (string, error) {
	if _, exists := mc.consumers[consumerID]; !exists {
		return "", fmt.Errorf("consumer with ID %s not found", consumerID)
	}

	apiKey, err := mc.keyGen.GenerateAPIKey("gw")
	if err != nil {
		return "", err
	}

	mc.apiKeys[consumerID] = append(mc.apiKeys[consumerID], apiKey)
	return apiKey, nil
}

// RevokeAPIKey revokes a mock API key for a consumer
func (mc *MockClient) RevokeAPIKey(consumerID, apiKey string) error {
	if _, exists := mc.consumers[consumerID]; !exists {
		return fmt.Errorf("consumer with ID %s not found", consumerID)
	}

	keys := mc.apiKeys[consumerID]
	for i, key := range keys {
		if key == apiKey {
			// Remove the key from the slice
			mc.apiKeys[consumerID] = append(keys[:i], keys[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("API key not found for consumer %s", consumerID)
}

// GetConsumer retrieves a mock consumer by ID
func (mc *MockClient) GetConsumer(consumerID string) (*Consumer, error) {
	consumer, exists := mc.consumers[consumerID]
	if !exists {
		return nil, fmt.Errorf("consumer with ID %s not found", consumerID)
	}

	// Return a copy to avoid external modifications
	result := *consumer
	return &result, nil
}

// UpdateConsumer updates a mock consumer
func (mc *MockClient) UpdateConsumer(consumerID string, req *CreateConsumerRequest) (*Consumer, error) {
	consumer, exists := mc.consumers[consumerID]
	if !exists {
		return nil, fmt.Errorf("consumer with ID %s not found", consumerID)
	}

	// Update fields
	consumer.Name = req.Name
	consumer.Enabled = req.Enabled
	consumer.Metadata = req.Metadata
	consumer.RateLimit = req.RateLimit
	consumer.IPWhitelist = req.IPWhitelist
	consumer.UpdatedAt = time.Now()

	mc.consumers[consumerID] = consumer

	// Return a copy
	result := *consumer
	return &result, nil
}

// Health always returns success for mock client
func (mc *MockClient) Health() error {
	return nil
}

// GetConsumers returns all mock consumers (for testing purposes)
func (mc *MockClient) GetConsumers() map[string]*Consumer {
	result := make(map[string]*Consumer)
	for id, consumer := range mc.consumers {
		// Return copies to avoid external modifications
		copy := *consumer
		result[id] = &copy
	}
	return result
}

// GetAPIKeys returns all API keys for a consumer (for testing purposes)
func (mc *MockClient) GetAPIKeys(consumerID string) []string {
	keys := mc.apiKeys[consumerID]
	if keys == nil {
		return []string{}
	}
	
	// Return a copy to avoid external modifications
	result := make([]string, len(keys))
	copy(result, keys)
	return result
}

// Reset clears all mock data (for testing purposes)
func (mc *MockClient) Reset() {
	mc.consumers = make(map[string]*Consumer)
	mc.apiKeys = make(map[string][]string)
}
