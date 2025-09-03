package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/metrics/driver/prometheus"
	"github.com/songzhibin97/stargate/pkg/metrics"
)

func TestNewMetricsMiddleware(t *testing.T) {
	// Create Prometheus provider
	provider, err := prometheus.NewProvider(prometheus.Options{
		Namespace: "test",
		Subsystem: "middleware",
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Create metrics middleware
	config := DefaultMetricsConfig()
	middleware, err := NewMetricsMiddleware(config, provider)
	if err != nil {
		t.Fatalf("Failed to create metrics middleware: %v", err)
	}

	if middleware == nil {
		t.Fatal("Expected middleware to be created")
	}

	// Test provider access
	if middleware.GetProvider() != provider {
		t.Error("Expected provider to match")
	}

	// Clean up
	middleware.Close()
}

func TestMetricsMiddlewareFromPrometheusConfig(t *testing.T) {
	// Test backward compatibility
	prometheusConfig := &config.PrometheusConfig{
		Enabled:   true,
		Namespace: "test",
		Subsystem: "compat",
	}

	middleware, err := NewMetricsMiddlewareFromPrometheusConfig(prometheusConfig)
	if err != nil {
		t.Fatalf("Failed to create middleware from Prometheus config: %v", err)
	}

	if middleware == nil {
		t.Fatal("Expected middleware to be created")
	}

	// Test that provider is working
	provider := middleware.GetProvider()
	if provider == nil {
		t.Fatal("Expected provider to be available")
	}

	if provider.Name() != "prometheus" {
		t.Errorf("Expected provider name 'prometheus', got '%s'", provider.Name())
	}

	// Clean up
	middleware.Close()
}

func TestMetricsMiddlewareHTTPHandler(t *testing.T) {
	// Create middleware
	provider, err := prometheus.NewProvider(prometheus.Options{
		Namespace: "test",
		Subsystem: "http",
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	config := DefaultMetricsConfig()
	middleware, err := NewMetricsMiddleware(config, provider)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	defer middleware.Close()

	// Create test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
	})

	// Wrap with metrics middleware
	wrappedHandler := middleware.Handler()(testHandler)

	// Test request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if body := w.Body.String(); body != "Hello, World!" {
		t.Errorf("Expected 'Hello, World!', got '%s'", body)
	}

	// Test metrics endpoint
	metricsReq := httptest.NewRequest("GET", "/metrics", nil)
	metricsW := httptest.NewRecorder()

	provider.Handler().ServeHTTP(metricsW, metricsReq)

	if metricsW.Code != http.StatusOK {
		t.Errorf("Expected metrics status 200, got %d", metricsW.Code)
	}

	metricsBody := metricsW.Body.String()
	if !strings.Contains(metricsBody, "test_http_http_requests_total") {
		t.Error("Expected to find request counter metric in metrics output")
	}
}

func TestPrometheusMiddlewareAdapter(t *testing.T) {
	// Test backward compatibility adapter
	prometheusConfig := &config.PrometheusConfig{
		Enabled:   true,
		Namespace: "test",
		Subsystem: "adapter",
	}

	adapter, err := NewPrometheusMiddlewareAdapter(prometheusConfig)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}

	if adapter == nil {
		t.Fatal("Expected adapter to be created")
	}

	// Test that it behaves like the old PrometheusMiddleware
	metrics := adapter.GetMetrics()
	if metrics["enabled"] != true {
		t.Error("Expected enabled to be true")
	}
	if metrics["namespace"] != "test" {
		t.Error("Expected namespace to be 'test'")
	}
	if metrics["subsystem"] != "adapter" {
		t.Error("Expected subsystem to be 'adapter'")
	}

	// Test HTTP handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	wrappedHandler := adapter.Handler()(testHandler)
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Clean up
	adapter.Close()
}

func TestMetricsConfigValidation(t *testing.T) {
	// Test valid config
	validConfig := DefaultMetricsConfig()
	if err := ValidateMetricsConfig(validConfig); err != nil {
		t.Errorf("Expected valid config to pass validation: %v", err)
	}

	// Test invalid sample rate
	invalidConfig := DefaultMetricsConfig()
	invalidConfig.SampleRate = 1.5
	if err := ValidateMetricsConfig(invalidConfig); err == nil {
		t.Error("Expected invalid sample rate to fail validation")
	}

	// Test invalid buffer size with async updates
	invalidConfig2 := DefaultMetricsConfig()
	invalidConfig2.AsyncUpdates = true
	invalidConfig2.BufferSize = 0
	if err := ValidateMetricsConfig(invalidConfig2); err == nil {
		t.Error("Expected invalid buffer size to fail validation")
	}

	// Test invalid const labels
	invalidConfig3 := DefaultMetricsConfig()
	invalidConfig3.ConstLabels = map[string]string{
		"invalid-label": "value",
	}
	if err := ValidateMetricsConfig(invalidConfig3); err == nil {
		t.Error("Expected invalid label name to fail validation")
	}
}

func TestMigratePrometheusConfig(t *testing.T) {
	// Test migration from old config
	oldConfig := &config.PrometheusConfig{
		Enabled:   true,
		Namespace: "old",
		Subsystem: "config",
	}

	newConfig := MigratePrometheusConfig(oldConfig)

	if newConfig.Enabled != oldConfig.Enabled {
		t.Error("Expected enabled to be migrated")
	}
	if newConfig.Namespace != oldConfig.Namespace {
		t.Error("Expected namespace to be migrated")
	}
	if newConfig.Subsystem != oldConfig.Subsystem {
		t.Error("Expected subsystem to be migrated")
	}
	if newConfig.Provider != "prometheus" {
		t.Error("Expected provider to be set to prometheus")
	}

	// Test migration with nil config
	nilConfig := MigratePrometheusConfig(nil)
	if nilConfig == nil {
		t.Error("Expected default config when migrating nil")
	}
}

func TestMetricsProviderFactory(t *testing.T) {
	factory := &MetricsProviderFactory{}

	// Test supported providers
	providers := factory.GetSupportedProviders()
	if len(providers) == 0 {
		t.Error("Expected at least one supported provider")
	}

	found := false
	for _, provider := range providers {
		if provider == "prometheus" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected prometheus to be in supported providers")
	}

	// Test creating Prometheus provider
	provider, err := factory.CreateProvider("prometheus", metrics.ProviderOptions{
		Namespace: "test",
		Subsystem: "factory",
	})
	if err != nil {
		t.Fatalf("Failed to create prometheus provider: %v", err)
	}

	if provider.Name() != "prometheus" {
		t.Errorf("Expected provider name 'prometheus', got '%s'", provider.Name())
	}
}

func BenchmarkMetricsMiddleware(b *testing.B) {
	// Create middleware
	provider, err := prometheus.NewProvider(prometheus.Options{
		Namespace: "bench",
		Subsystem: "test",
	})
	if err != nil {
		b.Fatalf("Failed to create provider: %v", err)
	}

	config := DefaultMetricsConfig()
	middleware, err := NewMetricsMiddleware(config, provider)
	if err != nil {
		b.Fatalf("Failed to create middleware: %v", err)
	}
	defer middleware.Close()

	// Create test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware.Handler()(testHandler)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(w, req)
		}
	})
}
