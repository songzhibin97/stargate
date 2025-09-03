package prometheus

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/songzhibin97/stargate/pkg/metrics"
)

// Example demonstrates how to use the Prometheus provider
func ExamplePrometheusProvider() {
	// Create a new Prometheus provider
	provider, err := NewProvider(Options{
		Namespace: "example",
		Subsystem: "app",
	})
	if err != nil {
		panic(err)
	}

	// Create some metrics
	requestCounter, err := provider.NewCounterVec(metrics.MetricOptions{
		Name:   "http_requests_total",
		Help:   "Total number of HTTP requests",
		Labels: []string{"method", "status"},
	})
	if err != nil {
		panic(err)
	}

	responseTime, err := provider.NewHistogramVec(metrics.MetricOptions{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds",
		Labels:  []string{"method", "endpoint"},
		Buckets: []float64{0.1, 0.5, 1.0, 2.5, 5.0, 10.0},
	})
	if err != nil {
		panic(err)
	}

	activeConnections, err := provider.NewGauge(metrics.MetricOptions{
		Name: "active_connections",
		Help: "Number of active connections",
	})
	if err != nil {
		panic(err)
	}

	// Simulate some metrics
	requestCounter.WithLabelValues("GET", "200").Add(100)
	requestCounter.WithLabelValues("POST", "201").Add(50)
	requestCounter.WithLabelValues("GET", "404").Add(5)

	responseTime.WithLabelValues("GET", "/api/users").Observe(0.123)
	responseTime.WithLabelValues("GET", "/api/users").Observe(0.456)
	responseTime.WithLabelValues("POST", "/api/users").Observe(0.789)

	activeConnections.Set(42)

	// Create HTTP handler for metrics endpoint
	handler := provider.Handler()

	// Test the metrics endpoint
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	fmt.Printf("Status: %d\n", w.Code)
	fmt.Printf("Content-Type: %s\n", w.Header().Get("Content-Type"))
	
	// Check if metrics are present in the response
	body := w.Body.String()
	if len(body) > 0 {
		fmt.Println("Metrics endpoint is working!")
	}

	// Output:
	// Status: 200
	// Content-Type: text/plain; version=0.0.4; charset=utf-8; escaping=underscores
	// Metrics endpoint is working!
}

// Example demonstrates how to use the factory pattern
func ExampleFactory() {
	// Get the Prometheus factory
	factory, err := metrics.GetFactory("prometheus")
	if err != nil {
		panic(err)
	}

	// Create a provider using the factory
	provider, err := factory.Create(metrics.ProviderOptions{
		Namespace: "myapp",
		Subsystem: "metrics",
		ConstLabels: map[string]string{
			"version": "1.0.0",
			"env":     "production",
		},
	})
	if err != nil {
		panic(err)
	}

	// Create a counter
	counter, err := provider.NewCounter(metrics.MetricOptions{
		Name: "operations_total",
		Help: "Total number of operations",
	})
	if err != nil {
		panic(err)
	}

	// Use the counter
	counter.Inc()
	counter.Add(5)

	fmt.Printf("Counter value: %.0f\n", counter.Get())
	fmt.Printf("Provider name: %s\n", provider.Name())

	// Output:
	// Counter value: 6
	// Provider name: prometheus
}

// Example demonstrates how to use the builder pattern
func ExampleBuilder() {
	// Create a provider using the builder
	provider := NewBuilder().
		WithNamespace("stargate").
		WithSubsystem("gateway").
		WithConstLabels(map[string]string{
			"service": "gateway-01",
			"region":  "us-west-2",
		}).
		MustBuild()

	// Create metrics using the convenient factory
	factory := metrics.NewConvenientFactory(provider)

	// Create common HTTP metrics
	httpRequests := factory.MustCounterVec(
		"http_requests_total",
		"Total HTTP requests",
		[]string{"method", "status_code"},
	)

	httpDuration := factory.MustHistogramVec(
		"http_request_duration_seconds",
		"HTTP request duration",
		[]string{"method", "endpoint"},
	)

	// Simulate some requests
	httpRequests.WithLabelValues("GET", "200").Inc()
	httpRequests.WithLabelValues("POST", "201").Inc()

	httpDuration.WithLabelValues("GET", "/health").Observe(0.001)
	httpDuration.WithLabelValues("POST", "/api/data").Observe(0.123)

	// Gather metrics
	families, err := provider.Gather()
	if err != nil {
		panic(err)
	}

	fmt.Printf("Number of metric families: %d\n", len(families))
	fmt.Printf("Provider version: %s\n", provider.Version())

	// Output:
	// Number of metric families: 2
	// Provider version: 1.0.0
}

// Example demonstrates HTTP server integration
func ExampleProvider_Handler() {
	// Create provider
	provider := MustDefaultProviderWithOptions("webapp", "server", map[string]string{
		"service": "web-api",
	})

	// Create metrics
	requestsTotal, err := provider.NewCounterVec(metrics.MetricOptions{
		Name:   "requests_total",
		Help:   "Total requests",
		Labels: []string{"method", "path", "status"},
	})
	if err != nil {
		panic(err)
	}

	requestDuration, err := provider.NewHistogramVec(metrics.MetricOptions{
		Name:    "request_duration_seconds",
		Help:    "Request duration",
		Labels:  []string{"method", "path"},
		Buckets: metrics.GetDefaultBuckets("latency"),
	})
	if err != nil {
		panic(err)
	}

	// Create HTTP server with metrics endpoint
	mux := http.NewServeMux()

	// Add metrics endpoint
	mux.Handle("/metrics", provider.Handler())

	// Add a sample endpoint with metrics
	mux.HandleFunc("/api/hello", func(w http.ResponseWriter, r *http.Request) {
		// Record metrics
		requestsTotal.WithLabelValues(r.Method, r.URL.Path, "200").Inc()
		requestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(0.050)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
	})

	// Test the server
	server := httptest.NewServer(mux)
	defer server.Close()

	// Make a request to the API
	resp, err := http.Get(server.URL + "/api/hello")
	if err != nil {
		panic(err)
	}
	resp.Body.Close()

	// Check metrics endpoint
	metricsResp, err := http.Get(server.URL + "/metrics")
	if err != nil {
		panic(err)
	}
	defer metricsResp.Body.Close()

	fmt.Printf("API Status: %d\n", resp.StatusCode)
	fmt.Printf("Metrics Status: %d\n", metricsResp.StatusCode)
	fmt.Printf("Metrics Content-Type: %s\n", metricsResp.Header.Get("Content-Type"))

	// Output:
	// API Status: 200
	// Metrics Status: 200
	// Metrics Content-Type: text/plain; version=0.0.4; charset=utf-8; escaping=underscores
}
