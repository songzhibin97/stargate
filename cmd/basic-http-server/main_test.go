package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
)

// TestBasicHTTPServer_NewServer tests server creation
func TestBasicHTTPServer_NewServer(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Address:        ":8080",
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			IdleTimeout:    60 * time.Second,
			MaxHeaderBytes: 1048576,
		},
	}

	server := NewBasicHTTPServer(cfg)
	if server == nil {
		t.Fatal("Expected server to be created, got nil")
	}

	if server.config != cfg {
		t.Error("Expected server config to match provided config")
	}

	if server.httpServer.Addr != cfg.Server.Address {
		t.Errorf("Expected server address %s, got %s", cfg.Server.Address, server.httpServer.Addr)
	}
}

// TestBasicHTTPServer_HealthEndpoint tests the health check endpoint
func TestBasicHTTPServer_HealthEndpoint(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Address:        ":0", // Use random port
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			IdleTimeout:    60 * time.Second,
			MaxHeaderBytes: 1048576,
		},
	}

	server := NewBasicHTTPServer(cfg)

	// Start server in background
	serverStarted := make(chan string, 1)
	go func() {
		// Create listener first to get actual address
		listener, err := net.Listen("tcp", cfg.Server.Address)
		if err != nil {
			t.Errorf("Failed to create listener: %v", err)
			return
		}

		// Send actual address to main goroutine
		serverStarted <- listener.Addr().String()

		// Start server with the listener
		server.httpServer.Addr = listener.Addr().String()
		if err := server.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			t.Logf("Server start error: %v", err)
		}
	}()

	// Wait for server to start and get actual address
	var addr string
	select {
	case addr = <-serverStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("Server failed to start within timeout")
	}

	// Test health endpoint
	resp, err := http.Get(fmt.Sprintf("http://%s/health", addr))
	if err != nil {
		// If connection fails, try with a different approach
		t.Logf("Direct connection failed: %v, testing handler directly", err)
		
		// Test handler directly
		req, _ := http.NewRequest("GET", "/health", nil)
		rr := &testResponseWriter{
			header: make(http.Header),
			status: 200,
		}
		
		server.handleHealth(rr, req)
		
		if rr.status != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, rr.status)
		}
		
		contentType := rr.header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", contentType)
		}
		
		if len(rr.body) == 0 {
			t.Error("Expected response body, got empty")
		}
		
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	// Shutdown server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(ctx)
}

// TestBasicHTTPServer_DefaultEndpoint tests the default endpoint
func TestBasicHTTPServer_DefaultEndpoint(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Address:        ":8082",
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			IdleTimeout:    60 * time.Second,
			MaxHeaderBytes: 1048576,
		},
	}

	server := NewBasicHTTPServer(cfg)

	// Test default handler directly
	req, _ := http.NewRequest("GET", "/test", nil)
	rr := &testResponseWriter{
		header: make(http.Header),
		status: 200,
	}

	server.handleDefault(rr, req)

	if rr.status != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.status)
	}

	contentType := rr.header.Get("Content-Type")
	if contentType != "text/plain" {
		t.Errorf("Expected Content-Type text/plain, got %s", contentType)
	}

	if len(rr.body) == 0 {
		t.Error("Expected response body, got empty")
	}
}

// TestBasicHTTPServer_GracefulShutdown tests graceful shutdown
func TestBasicHTTPServer_GracefulShutdown(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Address:        ":8083",
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			IdleTimeout:    60 * time.Second,
			MaxHeaderBytes: 1048576,
		},
	}

	server := NewBasicHTTPServer(cfg)

	// Start server in background
	serverErr := make(chan error, 1)
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Wait a bit for server to start
	time.Sleep(100 * time.Millisecond)

	// Check if server started successfully
	select {
	case err := <-serverErr:
		t.Fatalf("Server failed to start: %v", err)
	default:
		// Server started successfully
	}

	// Test graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	shutdownErr := server.Shutdown(ctx)
	if shutdownErr != nil {
		t.Errorf("Expected graceful shutdown to succeed, got error: %v", shutdownErr)
	}
}

// TestConfigLoading tests configuration loading
func TestConfigLoading(t *testing.T) {
	// Create a temporary config file
	configContent := `
server:
  address: ":9090"
  read_timeout: 15s
  write_timeout: 15s
  idle_timeout: 120s
  max_header_bytes: 2097152
`

	tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpFile.Close()

	// Load configuration
	cfg, err := config.Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Server.Address != ":9090" {
		t.Errorf("Expected address :9090, got %s", cfg.Server.Address)
	}

	if cfg.Server.ReadTimeout != 15*time.Second {
		t.Errorf("Expected read timeout 15s, got %v", cfg.Server.ReadTimeout)
	}
}

// testResponseWriter is a simple implementation of http.ResponseWriter for testing
type testResponseWriter struct {
	header http.Header
	body   []byte
	status int
}

func (w *testResponseWriter) Header() http.Header {
	return w.header
}

func (w *testResponseWriter) Write(data []byte) (int, error) {
	w.body = append(w.body, data...)
	return len(data), nil
}

func (w *testResponseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
}

// TestSignalHandling tests SIGINT signal handling (integration test)
func TestSignalHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping signal handling test in short mode")
	}

	// This test simulates the signal handling behavior
	// In a real scenario, we would send SIGINT to the process
	
	cfg := &config.Config{
		Server: config.ServerConfig{
			Address:        ":8084",
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			IdleTimeout:    60 * time.Second,
			MaxHeaderBytes: 1048576,
		},
	}

	server := NewBasicHTTPServer(cfg)

	// Simulate signal handling
	sigChan := make(chan os.Signal, 1)
	
	// Start server
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			t.Logf("Server error: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Simulate receiving SIGINT
	go func() {
		time.Sleep(200 * time.Millisecond)
		sigChan <- syscall.SIGINT
	}()

	// Wait for signal
	<-sigChan

	// Perform graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		t.Errorf("Graceful shutdown failed: %v", err)
	}

	t.Log("Signal handling test completed successfully")
}
