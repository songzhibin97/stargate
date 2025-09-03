package file

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewFileSource(t *testing.T) {
	// Create a temporary file for testing
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	
	// Write initial config
	initialConfig := `
server:
  address: ":8080"
  timeout: 30s
routes:
  - name: "test-route"
    path: "/api/v1"
`
	err := os.WriteFile(configFile, []byte(initialConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	tests := []struct {
		name         string
		filePath     string
		pollInterval time.Duration
		expectError  bool
	}{
		{
			name:         "valid file path",
			filePath:     configFile,
			pollInterval: time.Second,
			expectError:  false,
		},
		{
			name:         "empty file path",
			filePath:     "",
			pollInterval: time.Second,
			expectError:  true,
		},
		{
			name:         "non-existent file",
			filePath:     "/non/existent/file.yaml",
			pollInterval: time.Second,
			expectError:  true,
		},
		{
			name:         "zero poll interval (should default to 1s)",
			filePath:     configFile,
			pollInterval: 0,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, err := NewFileSource(tt.filePath, tt.pollInterval)
			
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

func TestFileSource_Get(t *testing.T) {
	// Create a temporary file for testing
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	
	expectedContent := `
server:
  address: ":8080"
  timeout: 30s
logging:
  level: "info"
  format: "json"
`
	
	err := os.WriteFile(configFile, []byte(expectedContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	source, err := NewFileSource(configFile, time.Second)
	if err != nil {
		t.Fatalf("Failed to create file source: %v", err)
	}
	defer source.Close()

	// Test Get method
	data, err := source.Get()
	if err != nil {
		t.Errorf("Get() returned error: %v", err)
	}

	if string(data) != expectedContent {
		t.Errorf("Get() returned unexpected content.\nExpected: %s\nGot: %s", expectedContent, string(data))
	}
}

func TestFileSource_Watch(t *testing.T) {
	// Create a temporary file for testing
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	
	initialContent := `
server:
  address: ":8080"
  timeout: 30s
`
	
	err := os.WriteFile(configFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	source, err := NewFileSource(configFile, 100*time.Millisecond) // Fast polling for testing
	if err != nil {
		t.Fatalf("Failed to create file source: %v", err)
	}
	defer source.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start watching
	ch, err := source.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch() returned error: %v", err)
	}

	// Should receive initial content immediately
	select {
	case data := <-ch:
		if string(data) != initialContent {
			t.Errorf("Initial watch data mismatch.\nExpected: %s\nGot: %s", initialContent, string(data))
		}
	case <-time.After(1 * time.Second):
		t.Errorf("Timeout waiting for initial config data")
	}

	// Modify the file
	updatedContent := `
server:
  address: ":9090"
  timeout: 60s
logging:
  level: "debug"
`
	
	// Wait a bit to ensure different modification time
	time.Sleep(10 * time.Millisecond)
	
	err = os.WriteFile(configFile, []byte(updatedContent), 0644)
	if err != nil {
		t.Fatalf("Failed to update test config file: %v", err)
	}

	// Should receive updated content
	select {
	case data := <-ch:
		if string(data) != updatedContent {
			t.Errorf("Updated watch data mismatch.\nExpected: %s\nGot: %s", updatedContent, string(data))
		}
	case <-time.After(2 * time.Second):
		t.Errorf("Timeout waiting for updated config data")
	}
}

func TestFileSource_WatchContextCancellation(t *testing.T) {
	// Create a temporary file for testing
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	
	initialContent := `server: { address: ":8080" }`
	
	err := os.WriteFile(configFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	source, err := NewFileSource(configFile, time.Second)
	if err != nil {
		t.Fatalf("Failed to create file source: %v", err)
	}
	defer source.Close()

	ctx, cancel := context.WithCancel(context.Background())

	// Start watching
	ch, err := source.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch() returned error: %v", err)
	}

	// Receive initial content
	select {
	case <-ch:
		// Expected
	case <-time.After(1 * time.Second):
		t.Errorf("Timeout waiting for initial config data")
	}

	// Cancel context
	cancel()

	// Channel should be closed
	select {
	case _, ok := <-ch:
		if ok {
			t.Errorf("Expected channel to be closed after context cancellation")
		}
	case <-time.After(2 * time.Second):
		t.Errorf("Timeout waiting for channel to close")
	}
}

func TestFileSource_MultipleWatchers(t *testing.T) {
	// Create a temporary file for testing
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	
	initialContent := `test: value`
	
	err := os.WriteFile(configFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	source, err := NewFileSource(configFile, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to create file source: %v", err)
	}
	defer source.Close()

	ctx := context.Background()

	// Create multiple watchers
	ch1, err1 := source.Watch(ctx)
	ch2, err2 := source.Watch(ctx)
	
	if err1 != nil || err2 != nil {
		t.Fatalf("Failed to create watchers: %v, %v", err1, err2)
	}

	// Both should receive initial content
	timeout := time.After(1 * time.Second)
	
	select {
	case <-ch1:
		// Expected
	case <-timeout:
		t.Errorf("Timeout waiting for initial data on watcher 1")
	}
	
	select {
	case <-ch2:
		// Expected
	case <-timeout:
		t.Errorf("Timeout waiting for initial data on watcher 2")
	}
}
