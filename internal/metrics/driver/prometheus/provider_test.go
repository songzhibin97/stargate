package prometheus

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/songzhibin97/stargate/pkg/metrics"
)

func TestPrometheusProvider_NewCounter(t *testing.T) {
	provider, err := NewProvider(Options{
		Namespace: "test",
		Subsystem: "counter",
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	counter, err := provider.NewCounter(metrics.MetricOptions{
		Name: "requests_total",
		Help: "Total number of requests",
	})
	if err != nil {
		t.Fatalf("Failed to create counter: %v", err)
	}

	// Test counter operations
	counter.Inc()
	counter.Add(5.0)

	if got := counter.Get(); got != 6.0 {
		t.Errorf("Expected counter value 6.0, got %f", got)
	}
}

func TestPrometheusProvider_NewCounterVec(t *testing.T) {
	provider, err := NewProvider(Options{
		Namespace: "test",
		Subsystem: "counter_vec",
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	counterVec, err := provider.NewCounterVec(metrics.MetricOptions{
		Name:   "http_requests_total",
		Help:   "Total HTTP requests",
		Labels: []string{"method", "status"},
	})
	if err != nil {
		t.Fatalf("Failed to create counter vector: %v", err)
	}

	// Test counter vector operations
	counter1 := counterVec.WithLabelValues("GET", "200")
	counter1.Inc()
	counter1.Add(2.0)

	counter2 := counterVec.With(map[string]string{
		"method": "POST",
		"status": "201",
	})
	counter2.Inc()

	if got := counter1.Get(); got != 3.0 {
		t.Errorf("Expected counter1 value 3.0, got %f", got)
	}

	if got := counter2.Get(); got != 1.0 {
		t.Errorf("Expected counter2 value 1.0, got %f", got)
	}
}

func TestPrometheusProvider_NewGauge(t *testing.T) {
	provider, err := NewProvider(Options{
		Namespace: "test",
		Subsystem: "gauge",
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	gauge, err := provider.NewGauge(metrics.MetricOptions{
		Name: "active_connections",
		Help: "Number of active connections",
	})
	if err != nil {
		t.Fatalf("Failed to create gauge: %v", err)
	}

	// Test gauge operations
	gauge.Set(10.0)
	gauge.Inc()
	gauge.Add(5.0)
	gauge.Dec()
	gauge.Sub(2.0)

	if got := gauge.Get(); got != 13.0 {
		t.Errorf("Expected gauge value 13.0, got %f", got)
	}
}

func TestPrometheusProvider_NewHistogram(t *testing.T) {
	provider, err := NewProvider(Options{
		Namespace: "test",
		Subsystem: "histogram",
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	histogram, err := provider.NewHistogram(metrics.MetricOptions{
		Name:    "request_duration_seconds",
		Help:    "Request duration in seconds",
		Buckets: []float64{0.1, 0.5, 1.0, 2.5, 5.0, 10.0},
	})
	if err != nil {
		t.Fatalf("Failed to create histogram: %v", err)
	}

	// Test histogram operations
	histogram.Observe(0.5)
	histogram.Observe(1.5)
	histogram.Observe(3.0)

	if got := histogram.GetCount(); got != 3 {
		t.Errorf("Expected histogram count 3, got %d", got)
	}

	if got := histogram.GetSum(); got != 5.0 {
		t.Errorf("Expected histogram sum 5.0, got %f", got)
	}

	buckets := histogram.GetBuckets()
	if len(buckets) == 0 {
		t.Error("Expected histogram buckets, got none")
	}
}

func TestPrometheusProvider_HTTPHandler(t *testing.T) {
	provider, err := NewProvider(Options{
		Namespace: "test",
		Subsystem: "http",
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Create some metrics
	counter, err := provider.NewCounter(metrics.MetricOptions{
		Name: "requests_total",
		Help: "Total requests",
	})
	if err != nil {
		t.Fatalf("Failed to create counter: %v", err)
	}
	counter.Inc()

	gauge, err := provider.NewGauge(metrics.MetricOptions{
		Name: "active_connections",
		Help: "Active connections",
	})
	if err != nil {
		t.Fatalf("Failed to create gauge: %v", err)
	}
	gauge.Set(42.0)

	// Test HTTP handler
	handler := provider.Handler()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "test_http_requests_total") {
		t.Error("Expected counter metric in response")
	}

	if !strings.Contains(body, "test_http_active_connections") {
		t.Error("Expected gauge metric in response")
	}
}

func TestPrometheusProvider_Lifecycle(t *testing.T) {
	provider, err := NewProvider(Options{})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	ctx := context.Background()

	// Test start
	if err := provider.Start(ctx); err != nil {
		t.Errorf("Failed to start provider: %v", err)
	}

	// Test health check
	if err := provider.Health(); err != nil {
		t.Errorf("Health check failed: %v", err)
	}

	// Test stop
	if err := provider.Stop(ctx); err != nil {
		t.Errorf("Failed to stop provider: %v", err)
	}

	// Test health check after stop
	if err := provider.Health(); err == nil {
		t.Error("Expected health check to fail after stop")
	}
}

func TestPrometheusProvider_Gather(t *testing.T) {
	provider, err := NewProvider(Options{
		Namespace: "test",
		Subsystem: "gather",
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Create a counter
	counter, err := provider.NewCounter(metrics.MetricOptions{
		Name: "test_counter",
		Help: "Test counter",
	})
	if err != nil {
		t.Fatalf("Failed to create counter: %v", err)
	}
	counter.Inc()

	// Gather metrics
	families, err := provider.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	if len(families) == 0 {
		t.Error("Expected at least one metric family")
	}

	found := false
	for _, family := range families {
		if family.Name == "test_gather_test_counter" {
			found = true
			if family.Type != metrics.CounterType {
				t.Errorf("Expected counter type, got %v", family.Type)
			}
			if len(family.Metrics) != 1 {
				t.Errorf("Expected 1 metric, got %d", len(family.Metrics))
			}
			if family.Metrics[0].Value != 1.0 {
				t.Errorf("Expected metric value 1.0, got %f", family.Metrics[0].Value)
			}
			break
		}
	}

	if !found {
		t.Error("Expected to find test counter metric")
	}
}

func TestPrometheusProvider_ProviderInfo(t *testing.T) {
	provider, err := NewProvider(Options{})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	if name := provider.Name(); name != "prometheus" {
		t.Errorf("Expected provider name 'prometheus', got '%s'", name)
	}

	if version := provider.Version(); version == "" {
		t.Error("Expected non-empty version")
	}
}

func BenchmarkPrometheusProvider_CounterInc(b *testing.B) {
	provider, err := NewProvider(Options{})
	if err != nil {
		b.Fatalf("Failed to create provider: %v", err)
	}

	counter, err := provider.NewCounter(metrics.MetricOptions{
		Name: "benchmark_counter",
		Help: "Benchmark counter",
	})
	if err != nil {
		b.Fatalf("Failed to create counter: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			counter.Inc()
		}
	})
}

func BenchmarkPrometheusProvider_HistogramObserve(b *testing.B) {
	provider, err := NewProvider(Options{})
	if err != nil {
		b.Fatalf("Failed to create provider: %v", err)
	}

	histogram, err := provider.NewHistogram(metrics.MetricOptions{
		Name: "benchmark_histogram",
		Help: "Benchmark histogram",
	})
	if err != nil {
		b.Fatalf("Failed to create histogram: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			histogram.Observe(float64(time.Now().UnixNano()) / 1e9)
		}
	})
}
