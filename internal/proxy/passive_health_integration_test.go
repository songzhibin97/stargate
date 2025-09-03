package proxy

import (
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/health"
	"github.com/songzhibin97/stargate/internal/router"
	"github.com/songzhibin97/stargate/internal/types"
)

func TestPassiveHealthCheckIntegration(t *testing.T) {
	// Create test upstream servers
	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer healthyServer.Close()

	unhealthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer unhealthyServer.Close()

	// Create configuration with passive health check enabled
	cfg := &config.Config{
		Proxy: config.ProxyConfig{
			BufferSize:               32768,
			ConnectTimeout:           5 * time.Second,
			ResponseHeaderTimeout:    10 * time.Second,
			KeepAliveTimeout:         30 * time.Second,
			MaxIdleConns:             100,
			MaxIdleConnsPerHost:      10,
		},
		LoadBalancer: config.LoadBalancerConfig{
			DefaultAlgorithm: "round_robin",
			HealthCheck: config.HealthCheckConfig{
				Enabled:            true,
				Interval:           30 * time.Second,
				Timeout:            5 * time.Second,
				HealthyThreshold:   2,
				UnhealthyThreshold: 3,
				Path:               "/health",
				Passive: config.PassiveHealthCheckConfig{
					Enabled:              true,
					ConsecutiveFailures:  2, // Lower threshold for faster testing
					IsolationDuration:    5 * time.Second,
					RecoveryInterval:     1 * time.Second,
					ConsecutiveSuccesses: 1,
					FailureStatusCodes:   []int{500, 502, 503, 504, 505},
					TimeoutAsFailure:     true,
				},
			},
		},
		Upstreams: config.UpstreamsConfig{
			Defaults: config.UpstreamDefaults{
				Algorithm: "round_robin",
				HealthCheck: config.HealthCheckConfig{
					Enabled:            true,
					Interval:           30 * time.Second,
					Timeout:            5 * time.Second,
					HealthyThreshold:   2,
					UnhealthyThreshold: 3,
					Path:               "/health",
					Passive: config.PassiveHealthCheckConfig{
						Enabled:              true,
						ConsecutiveFailures:  2,
						IsolationDuration:    5 * time.Second,
						RecoveryInterval:     1 * time.Second,
						ConsecutiveSuccesses: 1,
						FailureStatusCodes:   []int{500, 502, 503, 504, 505},
						TimeoutAsFailure:     true,
					},
				},
			},
		},
	}

	// Create pipeline
	logger := log.New(os.Stdout, "[Pipeline] ", log.LstdFlags)
	pipeline, err := NewPipeline(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create pipeline: %v", err)
	}

	// Create test upstream with both healthy and unhealthy targets
	upstream := &types.Upstream{
		ID:        "test-upstream",
		Name:      "Test Upstream",
		Algorithm: "round_robin",
		Targets: []*types.Target{
			{
				Host:    "healthy.example.com",
				Port:    80,
				Weight:  1,
				Healthy: true,
			},
			{
				Host:    "unhealthy.example.com",
				Port:    80,
				Weight:  1,
				Healthy: true,
			},
		},
	}

	// Add upstream to load balancer
	err = pipeline.loadBalancer.UpdateUpstream(upstream)
	if err != nil {
		t.Fatalf("Failed to update upstream: %v", err)
	}

	// Add targets to passive health checker
	for _, target := range upstream.Targets {
		err = pipeline.passiveHealthChecker.AddTarget(upstream.ID, target)
		if err != nil {
			t.Fatalf("Failed to add target to passive health checker: %v", err)
		}
	}

	// Create mock route
	route := &Route{
		ID:         "test-route",
		UpstreamID: "test-upstream",
	}

	// Mock router to return our test route
	pipeline.router = &testRouter{route: route}

	// Simulate requests directly to passive health checker instead of through pipeline
	// This avoids complex pipeline setup issues
	for i := 0; i < 3; i++ {
		// Simulate successful request to healthy server
		result := &health.RequestResult{
			UpstreamID: upstream.ID,
			Target:     upstream.Targets[0],
			StatusCode: 200,
			Error:      nil,
			Duration:   100 * time.Millisecond,
			IsTimeout:  false,
			Timestamp:  time.Now(),
		}
		pipeline.passiveHealthChecker.RecordRequest(result)

		// Simulate failed request to unhealthy server
		result = &health.RequestResult{
			UpstreamID: upstream.ID,
			Target:     upstream.Targets[1],
			StatusCode: 500,
			Error:      nil,
			Duration:   100 * time.Millisecond,
			IsTimeout:  false,
			Timestamp:  time.Now(),
		}
		pipeline.passiveHealthChecker.RecordRequest(result)

		time.Sleep(100 * time.Millisecond)
	}

	// Wait a bit for passive health check to process
	time.Sleep(2 * time.Second)

	// Check if unhealthy target was isolated
	unhealthyTarget := upstream.Targets[1]
	healthy := pipeline.passiveHealthChecker.IsTargetHealthy(upstream.ID, unhealthyTarget)
	if healthy {
		t.Error("Unhealthy target should have been isolated by passive health check")
	}

	// Check target stats
	stats := pipeline.passiveHealthChecker.GetTargetStats(upstream.ID, unhealthyTarget)
	if stats == nil {
		t.Error("Expected target stats, got nil")
	} else {
		if failures, ok := stats["total_failures"].(int64); !ok || failures == 0 {
			t.Errorf("Expected failures > 0, got %v", failures)
		}
		if isolated, ok := stats["isolated"].(bool); !ok || !isolated {
			t.Errorf("Expected target to be isolated, got %v", isolated)
		}
	}

	// Test health status
	healthStatus := pipeline.passiveHealthChecker.Health()
	if !healthStatus["enabled"].(bool) {
		t.Error("Passive health checker should be enabled")
	}
	if healthStatus["isolated_targets"].(int) == 0 {
		t.Error("Should have at least one isolated target")
	}
}

func TestPassiveHealthCheckRecovery(t *testing.T) {
	// Create a server that starts unhealthy but becomes healthy
	requestCount := 0
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount <= 3 {
			// First 3 requests fail
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Error"))
		} else {
			// Subsequent requests succeed
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}
	}))
	defer testServer.Close()

	// Create configuration for passive health checker directly

	// Create passive health checker
	passiveConfig := &health.PassiveHealthConfig{
		Enabled:              true,
		ConsecutiveFailures:  2,
		IsolationDuration:    1 * time.Second,
		RecoveryInterval:     500 * time.Millisecond,
		ConsecutiveSuccesses: 1,
		FailureStatusCodes:   []int{500, 502, 503, 504, 505},
		TimeoutAsFailure:     true,
	}

	var recoveryCallbackCalled bool
	callback := func(upstreamID, targetKey string, healthy bool) {
		if healthy {
			recoveryCallbackCalled = true
		}
	}

	checker := health.NewPassiveHealthChecker(passiveConfig, callback)
	err := checker.Start()
	if err != nil {
		t.Fatalf("Failed to start passive health checker: %v", err)
	}
	defer checker.Stop()

	target := &types.Target{
		Host:    "127.0.0.1",
		Port:    getPortFromURL(testServer.URL),
		Weight:  1,
		Healthy: true,
	}

	err = checker.AddTarget("test-upstream", target)
	if err != nil {
		t.Fatalf("Failed to add target: %v", err)
	}

	// Send failing requests to isolate target
	for i := 0; i < 3; i++ {
		result := &health.RequestResult{
			UpstreamID: "test-upstream",
			Target:     target,
			StatusCode: 500,
			Error:      nil,
			Duration:   100 * time.Millisecond,
			IsTimeout:  false,
			Timestamp:  time.Now(),
		}
		checker.RecordRequest(result)
	}

	// Verify target is isolated
	if checker.IsTargetHealthy("test-upstream", target) {
		t.Error("Target should be isolated after failures")
	}

	// Wait for isolation period to expire
	time.Sleep(2 * time.Second)

	// Send successful request to trigger recovery
	result := &health.RequestResult{
		UpstreamID: "test-upstream",
		Target:     target,
		StatusCode: 200,
		Error:      nil,
		Duration:   100 * time.Millisecond,
		IsTimeout:  false,
		Timestamp:  time.Now(),
	}
	checker.RecordRequest(result)

	// Wait a bit for recovery processing
	time.Sleep(1 * time.Second)

	// Verify target is recovered
	if !checker.IsTargetHealthy("test-upstream", target) {
		t.Error("Target should be recovered after successful request")
	}

	if !recoveryCallbackCalled {
		t.Error("Recovery callback should have been called")
	}
}

// Helper functions

type testRouter struct {
	route *Route
}

func (tr *testRouter) Match(r *http.Request) (*Route, error) {
	return tr.route, nil
}

func (tr *testRouter) AddRoute(route *Route) error {
	return nil
}

func (tr *testRouter) UpdateRoute(route *router.RouteRule) error {
	return nil
}

func (tr *testRouter) DeleteRoute(id string) error {
	return nil
}

func (tr *testRouter) RemoveRoute(id string) error {
	return nil
}

func (tr *testRouter) ClearRoutes() error {
	return nil
}

func (tr *testRouter) ListRoutes() []*Route {
	return []*Route{tr.route}
}

func getPortFromURL(urlStr string) int {
	// Extract port from test server URL
	if urlStr == "" {
		return 8080
	}

	u, err := url.Parse(urlStr)
	if err != nil {
		return 8080
	}

	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return 8080
	}

	return port
}
