package loadbalancer

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/types"
)

// TestCanaryBalancer_HTTPIntegration tests canary deployment with actual HTTP requests
func TestCanaryBalancer_HTTPIntegration(t *testing.T) {
	// Create mock backend servers for different versions
	v1Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Version", "v1")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Response from V1"))
	}))
	defer v1Server.Close()

	v2Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Version", "v2")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Response from V2"))
	}))
	defer v2Server.Close()

	// Create canary balancer
	config := &config.Config{}
	cb := NewCanaryBalancer(config)

	// Setup canary group with 80% v1, 20% v2
	canaryConfig := &CanaryConfig{
		GroupID:  "api-service",
		Strategy: "weighted",
		Versions: []*CanaryVersionConfig{
			{
				Version:    "v1",
				UpstreamID: "api-v1",
				Weight:     80,
				Percentage: 80.0,
			},
			{
				Version:    "v2",
				UpstreamID: "api-v2",
				Weight:     20,
				Percentage: 20.0,
			},
		},
	}

	err := cb.UpdateCanaryGroup(canaryConfig)
	if err != nil {
		t.Fatalf("Failed to update canary group: %v", err)
	}

	// Create upstream configurations
	upstreamV1 := &types.Upstream{
		ID:   "api-v1",
		Name: "API Service V1",
		Targets: []*types.Target{
			{Host: extractHost(v1Server.URL), Port: extractPort(v1Server.URL), Healthy: true},
		},
		Metadata: map[string]string{
			"canary_group":   "api-service",
			"canary_version": "v1",
		},
	}

	upstreamV2 := &types.Upstream{
		ID:   "api-v2",
		Name: "API Service V2",
		Targets: []*types.Target{
			{Host: extractHost(v2Server.URL), Port: extractPort(v2Server.URL), Healthy: true},
		},
		Metadata: map[string]string{
			"canary_group":   "api-service",
			"canary_version": "v2",
		},
	}

	// Update upstreams
	err = cb.UpdateUpstream(upstreamV1)
	if err != nil {
		t.Fatalf("Failed to update upstream v1: %v", err)
	}

	err = cb.UpdateUpstream(upstreamV2)
	if err != nil {
		t.Fatalf("Failed to update upstream v2: %v", err)
	}

	// Test traffic distribution
	v1Count := 0
	v2Count := 0
	totalRequests := 1000

	for i := 0; i < totalRequests; i++ {
		// Select target using canary balancer
		target, err := cb.Select(&types.Upstream{ID: "api-service"})
		if err != nil {
			t.Fatalf("Failed to select target: %v", err)
		}

		// Make HTTP request to selected target
		url := fmt.Sprintf("http://%s:%d", target.Host, target.Port)
		resp, err := http.Get(url)
		if err != nil {
			t.Fatalf("Failed to make HTTP request: %v", err)
		}

		version := resp.Header.Get("X-Version")
		resp.Body.Close()

		switch version {
		case "v1":
			v1Count++
		case "v2":
			v2Count++
		default:
			t.Errorf("Unexpected version: %s", version)
		}
	}

	// Verify traffic distribution (allow 5% error margin)
	v1Percentage := float64(v1Count) / float64(totalRequests) * 100
	v2Percentage := float64(v2Count) / float64(totalRequests) * 100

	if v1Percentage < 75 || v1Percentage > 85 {
		t.Errorf("V1 should receive ~80%% traffic, got %.2f%%", v1Percentage)
	}

	if v2Percentage < 15 || v2Percentage > 25 {
		t.Errorf("V2 should receive ~20%% traffic, got %.2f%%", v2Percentage)
	}

	t.Logf("Traffic distribution - V1: %.2f%%, V2: %.2f%%", v1Percentage, v2Percentage)
}

// TestCanaryBalancer_GradualRollout tests gradual rollout scenario
func TestCanaryBalancer_GradualRollout(t *testing.T) {
	// Create mock servers
	stableServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Version", "stable")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Stable version"))
	}))
	defer stableServer.Close()

	canaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Version", "canary")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Canary version"))
	}))
	defer canaryServer.Close()

	config := &config.Config{}
	cb := NewCanaryBalancer(config)

	// Phase 1: 95% stable, 5% canary
	t.Run("Phase1_5PercentCanary", func(t *testing.T) {
		canaryConfig := &CanaryConfig{
			GroupID:  "gradual-rollout",
			Strategy: "percentage",
			Versions: []*CanaryVersionConfig{
				{
					Version:    "stable",
					UpstreamID: "stable-upstream",
					Percentage: 95.0,
				},
				{
					Version:    "canary",
					UpstreamID: "canary-upstream",
					Percentage: 5.0,
				},
			},
		}

		err := cb.UpdateCanaryGroup(canaryConfig)
		if err != nil {
			t.Fatalf("Failed to update canary group: %v", err)
		}

		// Setup upstreams
		setupUpstreams(t, cb, stableServer, canaryServer)

		// Test traffic distribution
		stableCount, canaryCount := testTrafficDistribution(t, cb, 1000)
		stablePercentage := float64(stableCount) / 10.0
		canaryPercentage := float64(canaryCount) / 10.0

		if stablePercentage < 90 || stablePercentage > 100 {
			t.Errorf("Phase 1: Stable should get ~95%%, got %.1f%%", stablePercentage)
		}

		if canaryPercentage > 10 {
			t.Errorf("Phase 1: Canary should get ~5%%, got %.1f%%", canaryPercentage)
		}

		t.Logf("Phase 1 - Stable: %.1f%%, Canary: %.1f%%", stablePercentage, canaryPercentage)
	})

	// Phase 2: 50% stable, 50% canary
	t.Run("Phase2_50PercentCanary", func(t *testing.T) {
		canaryConfig := &CanaryConfig{
			GroupID:  "gradual-rollout",
			Strategy: "percentage",
			Versions: []*CanaryVersionConfig{
				{
					Version:    "stable",
					UpstreamID: "stable-upstream",
					Percentage: 50.0,
				},
				{
					Version:    "canary",
					UpstreamID: "canary-upstream",
					Percentage: 50.0,
				},
			},
		}

		err := cb.UpdateCanaryGroup(canaryConfig)
		if err != nil {
			t.Fatalf("Failed to update canary group: %v", err)
		}

		// Re-setup upstreams after config update
		setupUpstreams(t, cb, stableServer, canaryServer)

		// Test traffic distribution
		stableCount, canaryCount := testTrafficDistribution(t, cb, 1000)
		stablePercentage := float64(stableCount) / 10.0
		canaryPercentage := float64(canaryCount) / 10.0

		if stablePercentage < 45 || stablePercentage > 55 {
			t.Errorf("Phase 2: Stable should get ~50%%, got %.1f%%", stablePercentage)
		}

		if canaryPercentage < 45 || canaryPercentage > 55 {
			t.Errorf("Phase 2: Canary should get ~50%%, got %.1f%%", canaryPercentage)
		}

		t.Logf("Phase 2 - Stable: %.1f%%, Canary: %.1f%%", stablePercentage, canaryPercentage)
	})

	// Phase 3: 100% canary (complete rollout)
	t.Run("Phase3_100PercentCanary", func(t *testing.T) {
		canaryConfig := &CanaryConfig{
			GroupID:  "gradual-rollout",
			Strategy: "percentage",
			Versions: []*CanaryVersionConfig{
				{
					Version:    "canary",
					UpstreamID: "canary-upstream",
					Percentage: 100.0,
				},
			},
		}

		err := cb.UpdateCanaryGroup(canaryConfig)
		if err != nil {
			t.Fatalf("Failed to update canary group: %v", err)
		}

		// Re-setup canary upstream after config update
		upstreamCanary := &types.Upstream{
			ID:   "canary-upstream",
			Name: "Canary Service",
			Targets: []*types.Target{
				{Host: extractHost(canaryServer.URL), Port: extractPort(canaryServer.URL), Healthy: true},
			},
			Metadata: map[string]string{
				"canary_group":   "gradual-rollout",
				"canary_version": "canary",
			},
		}

		err = cb.UpdateUpstream(upstreamCanary)
		if err != nil {
			t.Fatalf("Failed to update canary upstream: %v", err)
		}

		// Test traffic distribution
		stableCount, canaryCount := testTrafficDistribution(t, cb, 1000)
		canaryPercentage := float64(canaryCount) / 10.0

		if canaryPercentage < 95 {
			t.Errorf("Phase 3: Canary should get ~100%%, got %.1f%%", canaryPercentage)
		}

		if stableCount > 0 {
			t.Errorf("Phase 3: Stable should get 0%%, got %d requests", stableCount)
		}

		t.Logf("Phase 3 - Stable: %d, Canary: %.1f%%", stableCount, canaryPercentage)
	})
}

// TestCanaryBalancer_HealthyTargetsOnly tests that only healthy targets are selected
func TestCanaryBalancer_HealthyTargetsOnly(t *testing.T) {
	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Status", "healthy")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Healthy response"))
	}))
	defer healthyServer.Close()

	config := &config.Config{}
	cb := NewCanaryBalancer(config)

	// Create upstream with mixed healthy/unhealthy targets
	upstream := &types.Upstream{
		ID:   "mixed-health",
		Name: "Mixed Health Service",
		Targets: []*types.Target{
			{Host: extractHost(healthyServer.URL), Port: extractPort(healthyServer.URL), Healthy: true},
			{Host: "unhealthy-host", Port: 9999, Healthy: false},
		},
	}

	err := cb.UpdateUpstream(upstream)
	if err != nil {
		t.Fatalf("Failed to update upstream: %v", err)
	}

	// Test that only healthy targets are selected
	for i := 0; i < 100; i++ {
		target, err := cb.Select(upstream)
		if err != nil {
			t.Fatalf("Failed to select target: %v", err)
		}

		if !target.Healthy {
			t.Errorf("Selected unhealthy target: %s:%d", target.Host, target.Port)
		}

		if target.Host == "unhealthy-host" {
			t.Errorf("Selected unhealthy host: %s", target.Host)
		}
	}
}

// Helper functions

func setupUpstreams(t *testing.T, cb *CanaryBalancer, stableServer, canaryServer *httptest.Server) {
	upstreamStable := &types.Upstream{
		ID:   "stable-upstream",
		Name: "Stable Service",
		Targets: []*types.Target{
			{Host: extractHost(stableServer.URL), Port: extractPort(stableServer.URL), Healthy: true},
		},
		Metadata: map[string]string{
			"canary_group":   "gradual-rollout",
			"canary_version": "stable",
		},
	}

	upstreamCanary := &types.Upstream{
		ID:   "canary-upstream",
		Name: "Canary Service",
		Targets: []*types.Target{
			{Host: extractHost(canaryServer.URL), Port: extractPort(canaryServer.URL), Healthy: true},
		},
		Metadata: map[string]string{
			"canary_group":   "gradual-rollout",
			"canary_version": "canary",
		},
	}

	err := cb.UpdateUpstream(upstreamStable)
	if err != nil {
		t.Fatalf("Failed to update stable upstream: %v", err)
	}

	err = cb.UpdateUpstream(upstreamCanary)
	if err != nil {
		t.Fatalf("Failed to update canary upstream: %v", err)
	}
}

func testTrafficDistribution(t *testing.T, cb *CanaryBalancer, totalRequests int) (stableCount, canaryCount int) {
	for i := 0; i < totalRequests; i++ {
		target, err := cb.Select(&types.Upstream{ID: "gradual-rollout"})
		if err != nil {
			t.Fatalf("Failed to select target: %v", err)
		}

		url := fmt.Sprintf("http://%s:%d", target.Host, target.Port)
		resp, err := http.Get(url)
		if err != nil {
			t.Fatalf("Failed to make HTTP request: %v", err)
		}

		version := resp.Header.Get("X-Version")
		resp.Body.Close()

		switch version {
		case "stable":
			stableCount++
		case "canary":
			canaryCount++
		}
	}
	return stableCount, canaryCount
}

func extractHost(serverURL string) string {
	u, err := url.Parse(serverURL)
	if err != nil {
		return "127.0.0.1"
	}
	host := strings.Split(u.Host, ":")[0]
	return host
}

func extractPort(serverURL string) int {
	u, err := url.Parse(serverURL)
	if err != nil {
		return 8080
	}
	portStr := strings.Split(u.Host, ":")[1]
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 8080
	}
	return port
}
