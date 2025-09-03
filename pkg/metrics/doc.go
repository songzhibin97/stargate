// Package metrics provides a unified interface for metrics collection and reporting.
//
// This package abstracts metrics collection from specific implementations like Prometheus,
// StatsD, or other monitoring systems. It provides a clean, type-safe API for creating
// and managing metrics across different backends.
//
// # Basic Usage
//
// Create a provider and use it to create metrics:
//
//	// Create a provider (implementation-specific)
//	provider, err := prometheus.NewProvider(prometheus.Options{
//		Namespace: "myapp",
//		Subsystem: "api",
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Create metrics
//	requestCounter, err := provider.NewCounter(metrics.MetricOptions{
//		Name: "requests_total",
//		Help: "Total number of requests",
//		Labels: []string{"method", "status"},
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Use the metric
//	requestCounter.WithLabelValues("GET", "200").Inc()
//
// # Metric Types
//
// The package supports four main metric types:
//
// Counter: A monotonically increasing value
//
//	counter := provider.NewCounter(metrics.MetricOptions{
//		Name: "operations_total",
//		Help: "Total operations performed",
//	})
//	counter.Inc()        // Increment by 1
//	counter.Add(5.0)     // Add specific value
//
// Gauge: A value that can go up and down
//
//	gauge := provider.NewGauge(metrics.MetricOptions{
//		Name: "active_connections",
//		Help: "Number of active connections",
//	})
//	gauge.Set(42.0)      // Set to specific value
//	gauge.Inc()          // Increment by 1
//	gauge.Dec()          // Decrement by 1
//	gauge.Add(10.0)      // Add value
//	gauge.Sub(5.0)       // Subtract value
//
// Histogram: For measuring distributions
//
//	histogram := provider.NewHistogram(metrics.MetricOptions{
//		Name: "request_duration_seconds",
//		Help: "Request duration in seconds",
//		Buckets: []float64{0.1, 0.5, 1.0, 2.5, 5.0, 10.0},
//	})
//	histogram.Observe(1.23)  // Record observation
//
// Summary: For measuring distributions with quantiles
//
//	summary := provider.NewSummary(metrics.MetricOptions{
//		Name: "response_size_bytes",
//		Help: "Response size in bytes",
//		Objectives: map[float64]float64{
//			0.5:  0.05,   // 50th percentile with 5% error
//			0.9:  0.01,   // 90th percentile with 1% error
//			0.99: 0.001,  // 99th percentile with 0.1% error
//		},
//	})
//	summary.Observe(1024)
//
// # Vector Metrics
//
// Vector metrics allow you to create multiple time series with different label values:
//
//	counterVec := provider.NewCounterVec(metrics.MetricOptions{
//		Name: "http_requests_total",
//		Help: "Total HTTP requests",
//		Labels: []string{"method", "status_code"},
//	})
//
//	// Use with specific label values
//	counterVec.WithLabelValues("GET", "200").Inc()
//	counterVec.WithLabelValues("POST", "404").Inc()
//
//	// Or use with label map
//	counterVec.With(map[string]string{
//		"method": "PUT",
//		"status_code": "201",
//	}).Inc()
//
// # Convenient Factory
//
// For easier metric creation, use the convenient factory:
//
//	factory := metrics.NewConvenientFactory(provider)
//
//	// Create metrics with less boilerplate
//	counter := factory.MustCounter("requests_total", "Total requests")
//	gauge := factory.MustGauge("active_users", "Active users")
//	histogram := factory.MustHistogram("latency_seconds", "Request latency")
//
// # Common Metrics
//
// The package provides pre-defined common metrics:
//
//	common := metrics.NewCommonMetrics(factory)
//	
//	// Use common HTTP metrics
//	common.HTTPRequestsTotal.WithLabelValues("GET", "/api/users", "200").Inc()
//	common.HTTPRequestDuration.WithLabelValues("GET", "/api/users", "200").Observe(0.123)
//
// # Provider Registration
//
// Register and use different provider implementations:
//
//	// Register a provider factory
//	metrics.RegisterFactory("prometheus", prometheusFactory)
//	metrics.RegisterFactory("statsd", statsdFactory)
//
//	// Create provider by name
//	provider, err := metrics.NewProvider("prometheus", metrics.ProviderOptions{
//		Namespace: "myapp",
//	})
//
// # HTTP Handler
//
// Expose metrics via HTTP:
//
//	http.Handle("/metrics", provider.Handler())
//	log.Fatal(http.ListenAndServe(":8080", nil))
//
// # Error Handling
//
// The package provides structured error types:
//
//	counter, err := provider.NewCounter(opts)
//	if err != nil {
//		if metrics.IsValidationError(err) {
//			log.Printf("Validation error: %v", err)
//		} else if metrics.IsRegistrationError(err) {
//			log.Printf("Registration error: %v", err)
//		} else {
//			log.Printf("Other error: %v", err)
//		}
//	}
//
// # Best Practices
//
// 1. Use meaningful metric names that follow the format: namespace_subsystem_name_unit
// 2. Include helpful descriptions in the Help field
// 3. Use labels sparingly - high cardinality can impact performance
// 4. Validate metric names and labels using the provided validation functions
// 5. Use appropriate metric types for your use case
// 6. Consider using the convenient factory for simpler code
// 7. Handle errors appropriately, especially during initialization
//
// # Thread Safety
//
// All metric operations are thread-safe. You can safely use metrics from multiple
// goroutines without additional synchronization.
//
// # Performance Considerations
//
// - Metric creation is more expensive than metric updates
// - Create metrics once during initialization when possible
// - Be mindful of label cardinality - each unique combination creates a new time series
// - Use histograms for latency measurements, not summaries (unless you need quantiles)
// - Consider using sampling for high-frequency metrics in performance-critical paths
//
package metrics
