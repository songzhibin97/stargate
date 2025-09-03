package tls

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/songzhibin97/stargate/internal/config"
)

func TestNewACMEManager(t *testing.T) {
	tests := []struct {
		name    string
		config  *config.ACMEConfig
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "disabled config",
			config: &config.ACMEConfig{
				Enabled: false,
			},
			wantErr: true,
		},
		{
			name: "empty domains",
			config: &config.ACMEConfig{
				Enabled: true,
				Domains: []string{},
				Email:   "test@example.com",
			},
			wantErr: true,
		},
		{
			name: "empty email",
			config: &config.ACMEConfig{
				Enabled: true,
				Domains: []string{"example.com"},
				Email:   "",
			},
			wantErr: true,
		},
		{
			name: "TOS not accepted",
			config: &config.ACMEConfig{
				Enabled:   true,
				Domains:   []string{"example.com"},
				Email:     "test@example.com",
				AcceptTOS: false,
			},
			wantErr: true,
		},
		{
			name: "valid config",
			config: &config.ACMEConfig{
				Enabled:   true,
				Domains:   []string{"example.com", "www.example.com"},
				Email:     "test@example.com",
				CacheDir:  "/tmp/acme-test",
				AcceptTOS: true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewACMEManager(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewACMEManager() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && manager == nil {
				t.Error("NewACMEManager() returned nil manager for valid config")
			}
			if manager != nil {
				// Clean up test cache directory
				if tt.config.CacheDir != "" {
					os.RemoveAll(tt.config.CacheDir)
				}
			}
		})
	}
}

func TestACMEManager_StartStop(t *testing.T) {
	tempDir := t.TempDir()
	
	config := &config.ACMEConfig{
		Enabled:   true,
		Domains:   []string{"test.example.com"},
		Email:     "test@example.com",
		CacheDir:  tempDir,
		AcceptTOS: true,
	}

	manager, err := NewACMEManager(config)
	if err != nil {
		t.Fatalf("NewACMEManager() failed: %v", err)
	}

	// Test start
	err = manager.Start()
	if err != nil {
		t.Errorf("Start() failed: %v", err)
	}

	// Test double start
	err = manager.Start()
	if err == nil {
		t.Error("Start() should fail when already started")
	}

	// Test stop
	err = manager.Stop()
	if err != nil {
		t.Errorf("Stop() failed: %v", err)
	}

	// Test double stop
	err = manager.Stop()
	if err != nil {
		t.Errorf("Stop() should not fail when already stopped: %v", err)
	}
}

func TestACMEManager_GetTLSConfig(t *testing.T) {
	tempDir := t.TempDir()
	
	config := &config.ACMEConfig{
		Enabled:   true,
		Domains:   []string{"test.example.com"},
		Email:     "test@example.com",
		CacheDir:  tempDir,
		AcceptTOS: true,
	}

	manager, err := NewACMEManager(config)
	if err != nil {
		t.Fatalf("NewACMEManager() failed: %v", err)
	}

	tlsConfig := manager.GetTLSConfig()
	if tlsConfig == nil {
		t.Error("GetTLSConfig() returned nil")
	}

	if tlsConfig.GetCertificate == nil {
		t.Error("TLS config should have GetCertificate function")
	}

	if tlsConfig.MinVersion != tls.VersionTLS12 {
		t.Errorf("Expected MinVersion TLS 1.2, got %d", tlsConfig.MinVersion)
	}

	expectedProtos := []string{"h2", "http/1.1"}
	if len(tlsConfig.NextProtos) != len(expectedProtos) {
		t.Errorf("Expected %d protocols, got %d", len(expectedProtos), len(tlsConfig.NextProtos))
	}
}

func TestACMEManager_GetHTTPHandler(t *testing.T) {
	tempDir := t.TempDir()
	
	config := &config.ACMEConfig{
		Enabled:   true,
		Domains:   []string{"test.example.com"},
		Email:     "test@example.com",
		CacheDir:  tempDir,
		AcceptTOS: true,
	}

	manager, err := NewACMEManager(config)
	if err != nil {
		t.Fatalf("NewACMEManager() failed: %v", err)
	}

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Get ACME HTTP handler
	acmeHandler := manager.GetHTTPHandler(testHandler)
	if acmeHandler == nil {
		t.Error("GetHTTPHandler() returned nil")
	}

	// Test with a regular request (should pass through to test handler)
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	acmeHandler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestACMEManager_Getters(t *testing.T) {
	tempDir := t.TempDir()
	domains := []string{"test1.example.com", "test2.example.com"}
	
	config := &config.ACMEConfig{
		Enabled:   true,
		Domains:   domains,
		Email:     "test@example.com",
		CacheDir:  tempDir,
		AcceptTOS: true,
	}

	manager, err := NewACMEManager(config)
	if err != nil {
		t.Fatalf("NewACMEManager() failed: %v", err)
	}

	// Test GetDomains
	gotDomains := manager.GetDomains()
	if len(gotDomains) != len(domains) {
		t.Errorf("Expected %d domains, got %d", len(domains), len(gotDomains))
	}
	for i, domain := range domains {
		if gotDomains[i] != domain {
			t.Errorf("Expected domain %s, got %s", domain, gotDomains[i])
		}
	}

	// Test GetCacheDir
	gotCacheDir := manager.GetCacheDir()
	if gotCacheDir != tempDir {
		t.Errorf("Expected cache dir %s, got %s", tempDir, gotCacheDir)
	}

	// Test IsEnabled
	if !manager.IsEnabled() {
		t.Error("Expected IsEnabled() to return true")
	}
}

func TestACMEManager_Metrics(t *testing.T) {
	tempDir := t.TempDir()
	
	config := &config.ACMEConfig{
		Enabled:   true,
		Domains:   []string{"test.example.com"},
		Email:     "test@example.com",
		CacheDir:  tempDir,
		AcceptTOS: true,
	}

	manager, err := NewACMEManager(config)
	if err != nil {
		t.Fatalf("NewACMEManager() failed: %v", err)
	}

	metrics := manager.Metrics()
	if metrics == nil {
		t.Error("Metrics() returned nil")
	}

	// Check expected metrics fields
	expectedFields := []string{"enabled", "started", "domains", "cache_dir", "email", "domain_status"}
	for _, field := range expectedFields {
		if _, exists := metrics[field]; !exists {
			t.Errorf("Expected metrics field %s not found", field)
		}
	}

	// Check values
	if metrics["enabled"] != true {
		t.Error("Expected enabled to be true")
	}

	if metrics["started"] != false {
		t.Error("Expected started to be false before Start()")
	}

	if metrics["domains"] != 1 {
		t.Errorf("Expected 1 domain, got %v", metrics["domains"])
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *config.ACMEConfig
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "disabled config",
			config: &config.ACMEConfig{
				Enabled: false,
			},
			wantErr: false, // No validation needed if disabled
		},
		{
			name: "empty domains",
			config: &config.ACMEConfig{
				Enabled: true,
				Domains: []string{},
			},
			wantErr: true,
		},
		{
			name: "empty email",
			config: &config.ACMEConfig{
				Enabled: true,
				Domains: []string{"example.com"},
				Email:   "",
			},
			wantErr: true,
		},
		{
			name: "TOS not accepted",
			config: &config.ACMEConfig{
				Enabled:   true,
				Domains:   []string{"example.com"},
				Email:     "test@example.com",
				AcceptTOS: false,
			},
			wantErr: true,
		},
		{
			name: "relative cache dir",
			config: &config.ACMEConfig{
				Enabled:   true,
				Domains:   []string{"example.com"},
				Email:     "test@example.com",
				CacheDir:  "relative/path",
				AcceptTOS: true,
			},
			wantErr: true,
		},
		{
			name: "valid config",
			config: &config.ACMEConfig{
				Enabled:   true,
				Domains:   []string{"example.com"},
				Email:     "test@example.com",
				CacheDir:  "/absolute/path",
				AcceptTOS: true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
