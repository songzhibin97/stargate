package proxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/types"
)

// TestWebSocketProxy_IsWebSocketUpgrade tests WebSocket upgrade detection
func TestWebSocketProxy_IsWebSocketUpgrade(t *testing.T) {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{
			BufferSize: 32768,
		},
	}
	wp := NewWebSocketProxy(cfg)

	tests := []struct {
		name     string
		headers  map[string]string
		expected bool
	}{
		{
			name: "valid websocket upgrade",
			headers: map[string]string{
				"Connection":             "Upgrade",
				"Upgrade":                "websocket",
				"Sec-WebSocket-Version":  "13",
				"Sec-WebSocket-Key":      "dGhlIHNhbXBsZSBub25jZQ==",
			},
			expected: true,
		},
		{
			name: "missing connection header",
			headers: map[string]string{
				"Upgrade":               "websocket",
				"Sec-WebSocket-Version": "13",
				"Sec-WebSocket-Key":     "dGhlIHNhbXBsZSBub25jZQ==",
			},
			expected: false,
		},
		{
			name: "wrong upgrade value",
			headers: map[string]string{
				"Connection":            "Upgrade",
				"Upgrade":               "http2",
				"Sec-WebSocket-Version": "13",
				"Sec-WebSocket-Key":     "dGhlIHNhbXBsZSBub25jZQ==",
			},
			expected: false,
		},
		{
			name: "wrong websocket version",
			headers: map[string]string{
				"Connection":            "Upgrade",
				"Upgrade":               "websocket",
				"Sec-WebSocket-Version": "12",
				"Sec-WebSocket-Key":     "dGhlIHNhbXBsZSBub25jZQ==",
			},
			expected: false,
		},
		{
			name: "missing websocket key",
			headers: map[string]string{
				"Connection":            "Upgrade",
				"Upgrade":               "websocket",
				"Sec-WebSocket-Version": "13",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/ws", nil)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			result := wp.IsWebSocketUpgrade(req)
			if result != tt.expected {
				t.Errorf("IsWebSocketUpgrade() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestWebSocketProxy_GenerateAcceptKey tests WebSocket accept key generation
func TestWebSocketProxy_GenerateAcceptKey(t *testing.T) {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{
			BufferSize: 32768,
		},
	}
	wp := NewWebSocketProxy(cfg)

	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "standard test key",
			key:      "dGhlIHNhbXBsZSBub25jZQ==",
			expected: "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=",
		},
		{
			name:     "another test key",
			key:      "x3JJHMbDL1EzLkh9GBhXDw==",
			expected: "HSmrc0sMlYUkAGmm5OPpG2HaGWk=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wp.generateAcceptKey(tt.key)
			if result != tt.expected {
				t.Errorf("generateAcceptKey() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestWebSocketProxy_ConnectionManagement tests connection management
func TestWebSocketProxy_ConnectionManagement(t *testing.T) {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{
			BufferSize:       32768,
			ConnectTimeout:   5 * time.Second,
			KeepAliveTimeout: 30 * time.Second,
		},
	}
	wp := NewWebSocketProxy(cfg)

	// Test initial state
	if count := wp.GetActiveConnections(); count != 0 {
		t.Errorf("Initial active connections = %d, want 0", count)
	}

	// Create mock connections
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn1 := &websocketConnection{
		id:     "test1",
		ctx:    ctx,
		cancel: cancel,
	}

	conn2 := &websocketConnection{
		id:     "test2",
		ctx:    ctx,
		cancel: cancel,
	}

	// Add connections
	wp.mu.Lock()
	wp.activeConns["test1"] = conn1
	wp.activeConns["test2"] = conn2
	wp.mu.Unlock()

	// Test active connections count
	if count := wp.GetActiveConnections(); count != 2 {
		t.Errorf("Active connections after adding = %d, want 2", count)
	}

	// Test cleanup
	wp.cleanupConnection(conn1)

	if count := wp.GetActiveConnections(); count != 1 {
		t.Errorf("Active connections after cleanup = %d, want 1", count)
	}

	// Test close all
	err := wp.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	if count := wp.GetActiveConnections(); count != 0 {
		t.Errorf("Active connections after close = %d, want 0", count)
	}
}

// TestWebSocketProxy_Integration tests WebSocket proxy integration
func TestWebSocketProxy_Integration(t *testing.T) {
	// Skip integration test in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create test WebSocket server
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("Test server upgrade error: %v", err)
			return
		}
		defer conn.Close()

		// Echo messages back
		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				break
			}
			err = conn.WriteMessage(messageType, message)
			if err != nil {
				break
			}
		}
	}))
	defer testServer.Close()

	// Parse test server URL to get host and port
	testURL := strings.Replace(testServer.URL, "http://", "", 1)
	parts := strings.Split(testURL, ":")
	if len(parts) != 2 {
		t.Fatalf("Invalid test server URL: %s", testServer.URL)
	}
	host := parts[0]
	port := parts[1]

	// Create proxy configuration
	cfg := &config.Config{
		Proxy: config.ProxyConfig{
			BufferSize:       32768,
			ConnectTimeout:   5 * time.Second,
			KeepAliveTimeout: 30 * time.Second,
		},
	}

	// Create custom pipeline with target injection
	logger := log.New(os.Stdout, "[Pipeline] ", log.LstdFlags)
	pipeline, err := NewPipeline(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create pipeline: %v", err)
	}

	// Create a custom handler that sets the target
	customHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set target for WebSocket proxy
		target := &types.Target{
			Host:    host,
			Port:    parsePort(port),
			Weight:  1,
			Healthy: true,
		}
		r = SetTarget(r, target)

		// Call the pipeline
		pipeline.ServeHTTP(w, r)
	})

	// Create proxy server
	proxyServer := httptest.NewServer(customHandler)
	defer proxyServer.Close()

	// Test WebSocket connection through proxy
	proxyURL := strings.Replace(proxyServer.URL, "http://", "ws://", 1) + "/ws"

	// Connect to proxy
	conn, _, err := websocket.DefaultDialer.Dial(proxyURL, http.Header{
		"Origin": []string{proxyServer.URL},
	})
	if err != nil {
		t.Fatalf("Failed to connect to proxy: %v", err)
	}
	defer conn.Close()

	// Test message echo
	testMessage := "Hello WebSocket Proxy!"
	err = conn.WriteMessage(websocket.TextMessage, []byte(testMessage))
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Read response
	_, response, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if string(response) != testMessage {
		t.Errorf("Response = %s, want %s", string(response), testMessage)
	}
}

// parsePort converts string port to int
func parsePort(portStr string) int {
	port := 80
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		return 80
	}
	return port
}

// BenchmarkWebSocketProxy_IsWebSocketUpgrade benchmarks upgrade detection
func BenchmarkWebSocketProxy_IsWebSocketUpgrade(b *testing.B) {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{
			BufferSize: 32768,
		},
	}
	wp := NewWebSocketProxy(cfg)

	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wp.IsWebSocketUpgrade(req)
	}
}

// BenchmarkWebSocketProxy_GenerateAcceptKey benchmarks accept key generation
func BenchmarkWebSocketProxy_GenerateAcceptKey(b *testing.B) {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{
			BufferSize: 32768,
		},
	}
	wp := NewWebSocketProxy(cfg)

	key := "dGhlIHNhbXBsZSBub25jZQ=="

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wp.generateAcceptKey(key)
	}
}

// TestIsConnectionClosed tests connection closure detection
func TestIsConnectionClosed(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "connection reset error",
			err:      fmt.Errorf("connection reset by peer"),
			expected: true,
		},
		{
			name:     "broken pipe error",
			err:      fmt.Errorf("broken pipe"),
			expected: true,
		},
		{
			name:     "connection refused error",
			err:      fmt.Errorf("connection refused"),
			expected: true,
		},
		{
			name:     "closed network connection error",
			err:      fmt.Errorf("use of closed network connection"),
			expected: true,
		},
		{
			name:     "other error",
			err:      fmt.Errorf("some other error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isConnectionClosed(tt.err)
			if result != tt.expected {
				t.Errorf("isConnectionClosed() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestGetProto tests protocol detection
func TestGetProto(t *testing.T) {
	tests := []struct {
		name     string
		tls      bool
		expected string
	}{
		{
			name:     "HTTP request",
			tls:      false,
			expected: "http",
		},
		{
			name:     "HTTPS request",
			tls:      true,
			expected: "https",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.tls {
				req.TLS = &tls.ConnectionState{}
			}

			result := getProto(req)
			if result != tt.expected {
				t.Errorf("getProto() = %v, want %v", result, tt.expected)
			}
		})
	}
}
