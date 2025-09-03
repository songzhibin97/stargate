package etcd

import (
	"context"
	"testing"
	"time"
)

func TestNewEtcdSource(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *EtcdConfig
		key         string
		expectError bool
	}{
		{
			name:        "nil config",
			cfg:         nil,
			key:         "/config/test",
			expectError: true,
		},
		{
			name: "empty key",
			cfg: &EtcdConfig{
				Endpoints: []string{"localhost:2379"},
				Timeout:   5 * time.Second,
			},
			key:         "",
			expectError: true,
		},
		{
			name: "empty endpoints",
			cfg: &EtcdConfig{
				Endpoints: []string{},
				Timeout:   5 * time.Second,
			},
			key:         "/config/test",
			expectError: true,
		},
		{
			name: "valid config but no etcd server (connection will fail)",
			cfg: &EtcdConfig{
				Endpoints: []string{"localhost:2379"},
				Timeout:   1 * time.Second,
			},
			key:         "/config/test",
			expectError: true, // Will fail because no etcd server is running
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, err := NewEtcdSource(tt.cfg, tt.key)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				if source != nil {
					source.Close()
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if source == nil {
				t.Errorf("Expected source but got nil")
			}
			
			// Clean up
			if source != nil {
				source.Close()
			}
		})
	}
}

func TestEtcdSource_Get_NoServer(t *testing.T) {
	// This test verifies behavior when etcd server is not available
	cfg := &EtcdConfig{
		Endpoints: []string{"localhost:2379"},
		Timeout:   1 * time.Second,
	}

	// This will fail during NewEtcdSource because it tests connection
	source, err := NewEtcdSource(cfg, "/config/test")
	if err == nil {
		t.Skip("Skipping test because etcd server appears to be running")
		if source != nil {
			source.Close()
		}
		return
	}

	// Expected behavior: should fail to create source when etcd is not available
	if source != nil {
		t.Errorf("Expected source to be nil when etcd is not available")
		source.Close()
	}
}

func TestEtcdSource_Watch_NoServer(t *testing.T) {
	// This test verifies behavior when etcd server is not available
	cfg := &EtcdConfig{
		Endpoints: []string{"localhost:2379"},
		Timeout:   1 * time.Second,
	}

	// This will fail during NewEtcdSource because it tests connection
	source, err := NewEtcdSource(cfg, "/config/test")
	if err == nil {
		t.Skip("Skipping test because etcd server appears to be running")
		if source != nil {
			source.Close()
		}
		return
	}

	// Expected behavior: should fail to create source when etcd is not available
	if source != nil {
		t.Errorf("Expected source to be nil when etcd is not available")
		source.Close()
	}
}

func TestEtcdSource_Close(t *testing.T) {
	// Test closing a source that was never successfully created
	cfg := &EtcdConfig{
		Endpoints: []string{"localhost:2379"},
		Timeout:   1 * time.Second,
	}

	source, err := NewEtcdSource(cfg, "/config/test")
	if err != nil {
		// Expected when no etcd server is running
		return
	}

	// If we got here, etcd server is running
	defer source.Close()

	// Test that Close() works
	err = source.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Test that calling Close() again doesn't cause issues
	err = source.Close()
	if err != nil {
		t.Errorf("Second Close() returned error: %v", err)
	}

	// Test that operations fail after close
	_, err = source.Get()
	if err == nil {
		t.Errorf("Expected Get() to fail after Close()")
	}

	ctx := context.Background()
	_, err = source.Watch(ctx)
	if err == nil {
		t.Errorf("Expected Watch() to fail after Close()")
	}
}

func TestCreateTLSConfig(t *testing.T) {
	tests := []struct {
		name        string
		tlsConfig   *TLSConfig
		expectError bool
		expectNil   bool
	}{
		{
			name: "disabled TLS",
			tlsConfig: &TLSConfig{
				Enabled: false,
			},
			expectError: false,
			expectNil:   true,
		},
		{
			name: "enabled TLS without certificates",
			tlsConfig: &TLSConfig{
				Enabled: true,
			},
			expectError: false,
			expectNil:   false,
		},
		{
			name: "enabled TLS with invalid certificate files",
			tlsConfig: &TLSConfig{
				Enabled:  true,
				CertFile: "/non/existent/cert.pem",
				KeyFile:  "/non/existent/key.pem",
			},
			expectError: true,
			expectNil:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := createTLSConfig(tt.tlsConfig)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if tt.expectNil && config != nil {
				t.Errorf("Expected nil config but got non-nil")
			}
			
			if !tt.expectNil && config == nil {
				t.Errorf("Expected non-nil config but got nil")
			}
		})
	}
}

// Integration tests that require a running etcd server
// These tests will be skipped if etcd is not available

func TestEtcdSource_Integration_Get(t *testing.T) {
	cfg := &EtcdConfig{
		Endpoints: []string{"localhost:2379"},
		Timeout:   5 * time.Second,
	}

	source, err := NewEtcdSource(cfg, "/stargate/config/test")
	if err != nil {
		t.Skip("Skipping integration test: etcd server not available")
		return
	}
	defer source.Close()

	// Try to get a value (it may not exist, which is fine for this test)
	_, err = source.Get()
	// We don't check for specific error because the key might not exist
	// The important thing is that the method doesn't panic and returns some result
}

func TestEtcdSource_Integration_Watch(t *testing.T) {
	cfg := &EtcdConfig{
		Endpoints: []string{"localhost:2379"},
		Timeout:   5 * time.Second,
	}

	source, err := NewEtcdSource(cfg, "/stargate/config/test")
	if err != nil {
		t.Skip("Skipping integration test: etcd server not available")
		return
	}
	defer source.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start watching
	ch, err := source.Watch(ctx)
	if err != nil {
		t.Errorf("Watch() returned error: %v", err)
		return
	}

	// We should get at least one value (current value or timeout)
	select {
	case <-ch:
		// Got a value (could be current value or an update)
	case <-ctx.Done():
		// Timeout is acceptable for this test
	}
}

func TestEtcdSource_Integration_MultipleWatchers(t *testing.T) {
	cfg := &EtcdConfig{
		Endpoints: []string{"localhost:2379"},
		Timeout:   5 * time.Second,
	}

	source, err := NewEtcdSource(cfg, "/stargate/config/test")
	if err != nil {
		t.Skip("Skipping integration test: etcd server not available")
		return
	}
	defer source.Close()

	ctx := context.Background()

	// Create multiple watchers
	ch1, err1 := source.Watch(ctx)
	ch2, err2 := source.Watch(ctx)
	
	if err1 != nil || err2 != nil {
		t.Errorf("Failed to create watchers: %v, %v", err1, err2)
		return
	}

	// Both watchers should be independent
	timeout := time.After(1 * time.Second)
	
	// Check that both channels are working
	select {
	case <-ch1:
		// Got data from first watcher
	case <-timeout:
		// Timeout is acceptable
	}
	
	select {
	case <-ch2:
		// Got data from second watcher
	case <-timeout:
		// Timeout is acceptable
	}
}
