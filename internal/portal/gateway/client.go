package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
)

// Client represents a client for interacting with the data plane gateway Admin API
type Client struct {
	config     *config.Config
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// Consumer represents a gateway consumer
type Consumer struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	APIKey      string            `json:"api_key"`
	Enabled     bool              `json:"enabled"`
	Metadata    map[string]string `json:"metadata"`
	RateLimit   *RateLimitConfig  `json:"rate_limit,omitempty"`
	IPWhitelist []string          `json:"ip_whitelist,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// RateLimitConfig represents rate limiting configuration
type RateLimitConfig struct {
	RequestsPerSecond int           `json:"requests_per_second"`
	BurstSize         int           `json:"burst_size"`
	WindowSize        time.Duration `json:"window_size"`
}

// CreateConsumerRequest represents a request to create a consumer
type CreateConsumerRequest struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Enabled     bool              `json:"enabled"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	RateLimit   *RateLimitConfig  `json:"rate_limit,omitempty"`
	IPWhitelist []string          `json:"ip_whitelist,omitempty"`
}

// APIKeyResponse represents an API key generation response
type APIKeyResponse struct {
	APIKey    string    `json:"api_key"`
	CreatedAt time.Time `json:"created_at"`
}

// ErrorResponse represents an error response from the gateway
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// NewClient creates a new gateway client
func NewClient(cfg *config.Config) *Client {
	// Default to localhost data plane if not configured
	baseURL := "http://localhost:8080"
	if cfg.Gateway.DataPlaneURL != "" {
		baseURL = cfg.Gateway.DataPlaneURL
	}

	// Get API key for data plane authentication
	apiKey := cfg.Gateway.AdminAPIKey

	return &Client{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: baseURL,
		apiKey:  apiKey,
	}
}

// CreateConsumer creates a new consumer in the gateway
func (c *Client) CreateConsumer(consumerID, name string, metadata map[string]string) (*Consumer, error) {
	req := &CreateConsumerRequest{
		ID:       consumerID,
		Name:     name,
		Enabled:  true,
		Metadata: metadata,
		RateLimit: &RateLimitConfig{
			RequestsPerSecond: 100,
			BurstSize:         200,
			WindowSize:        time.Minute,
		},
	}

	// Make HTTP request to create consumer
	url := fmt.Sprintf("%s/admin/consumers", c.baseURL)
	respData, err := c.makeRequest("POST", url, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}

	var consumer Consumer
	if err := json.Unmarshal(respData, &consumer); err != nil {
		return nil, fmt.Errorf("failed to parse consumer response: %w", err)
	}

	// Generate API key for the consumer
	apiKey, err := c.GenerateAPIKey(consumerID)
	if err != nil {
		// If API key generation fails, still return the consumer
		// The API key can be generated later
		consumer.APIKey = ""
	} else {
		consumer.APIKey = apiKey
	}

	return &consumer, nil
}

// DeleteConsumer deletes a consumer from the gateway
func (c *Client) DeleteConsumer(consumerID string) error {
	url := fmt.Sprintf("%s/admin/consumers/%s", c.baseURL, consumerID)
	_, err := c.makeRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to delete consumer: %w", err)
	}
	return nil
}

// GenerateAPIKey generates a new API key for a consumer
func (c *Client) GenerateAPIKey(consumerID string) (string, error) {
	url := fmt.Sprintf("%s/admin/consumers/%s/api-keys", c.baseURL, consumerID)
	respData, err := c.makeRequest("POST", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate API key: %w", err)
	}

	var response APIKeyResponse
	if err := json.Unmarshal(respData, &response); err != nil {
		return "", fmt.Errorf("failed to parse API key response: %w", err)
	}

	return response.APIKey, nil
}

// RevokeAPIKey revokes an API key for a consumer
func (c *Client) RevokeAPIKey(consumerID, apiKey string) error {
	url := fmt.Sprintf("%s/admin/consumers/%s/api-keys/%s", c.baseURL, consumerID, apiKey)
	_, err := c.makeRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to revoke API key: %w", err)
	}
	return nil
}

// GetConsumer retrieves a consumer by ID
func (c *Client) GetConsumer(consumerID string) (*Consumer, error) {
	url := fmt.Sprintf("%s/admin/consumers/%s", c.baseURL, consumerID)
	respData, err := c.makeRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get consumer: %w", err)
	}

	var consumer Consumer
	if err := json.Unmarshal(respData, &consumer); err != nil {
		return nil, fmt.Errorf("failed to parse consumer response: %w", err)
	}

	return &consumer, nil
}

// UpdateConsumer updates a consumer
func (c *Client) UpdateConsumer(consumerID string, req *CreateConsumerRequest) (*Consumer, error) {
	url := fmt.Sprintf("%s/admin/consumers/%s", c.baseURL, consumerID)
	respData, err := c.makeRequest("PUT", url, req)
	if err != nil {
		return nil, fmt.Errorf("failed to update consumer: %w", err)
	}

	var consumer Consumer
	if err := json.Unmarshal(respData, &consumer); err != nil {
		return nil, fmt.Errorf("failed to parse consumer response: %w", err)
	}

	return &consumer, nil
}

// makeRequest makes an HTTP request to the gateway API
func (c *Client) makeRequest(method, url string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-Admin-Key", c.apiKey)
	}

	// Make request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for error status codes
	if resp.StatusCode >= 400 {
		var errorResp ErrorResponse
		if err := json.Unmarshal(respData, &errorResp); err == nil {
			return nil, fmt.Errorf("gateway API error (%d): %s - %s", resp.StatusCode, errorResp.Error, errorResp.Message)
		}
		return nil, fmt.Errorf("gateway API error (%d): %s", resp.StatusCode, string(respData))
	}

	return respData, nil
}

// Health checks the health of the gateway
func (c *Client) Health() error {
	url := fmt.Sprintf("%s/health", c.baseURL)
	_, err := c.makeRequest("GET", url, nil)
	return err
}
