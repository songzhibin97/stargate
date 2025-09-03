package health

import (
	"testing"
	"time"

	"github.com/songzhibin97/stargate/internal/types"
)

func TestPassiveHealthChecker_NewPassiveHealthChecker(t *testing.T) {
	config := &PassiveHealthConfig{
		Enabled:              true,
		ConsecutiveFailures:  3,
		IsolationDuration:    30 * time.Second,
		RecoveryInterval:     10 * time.Second,
		ConsecutiveSuccesses: 2,
		FailureStatusCodes:   []int{500, 502, 503},
		TimeoutAsFailure:     true,
	}

	callback := func(upstreamID, targetKey string, healthy bool) {
		// Callback for testing
	}

	checker := NewPassiveHealthChecker(config, callback)

	if checker == nil {
		t.Fatal("Expected non-nil checker")
	}

	if checker.config != config {
		t.Error("Config not set correctly")
	}

	if checker.callback == nil {
		t.Error("Callback not set")
	}
}

func TestPassiveHealthChecker_AddRemoveTarget(t *testing.T) {
	config := &PassiveHealthConfig{
		Enabled:              true,
		ConsecutiveFailures:  3,
		IsolationDuration:    30 * time.Second,
		RecoveryInterval:     10 * time.Second,
		ConsecutiveSuccesses: 2,
		FailureStatusCodes:   []int{500, 502, 503},
		TimeoutAsFailure:     true,
	}

	checker := NewPassiveHealthChecker(config, nil)
	
	target := &types.Target{
		Host:    "example.com",
		Port:    80,
		Healthy: true,
	}

	// Test adding target
	err := checker.AddTarget("upstream1", target)
	if err != nil {
		t.Fatalf("Failed to add target: %v", err)
	}

	// Check if target was added
	targetKey := "upstream1:example.com:80"
	checker.mu.RLock()
	state, exists := checker.targets[targetKey]
	checker.mu.RUnlock()

	if !exists {
		t.Fatal("Target was not added")
	}

	if state.upstreamID != "upstream1" {
		t.Error("Upstream ID not set correctly")
	}

	if state.target != target {
		t.Error("Target not set correctly")
	}

	if !state.healthy {
		t.Error("Target should be initially healthy")
	}

	// Test removing target
	err = checker.RemoveTarget("upstream1", target)
	if err != nil {
		t.Fatalf("Failed to remove target: %v", err)
	}

	// Check if target was removed
	checker.mu.RLock()
	_, exists = checker.targets[targetKey]
	checker.mu.RUnlock()

	if exists {
		t.Fatal("Target was not removed")
	}
}

func TestPassiveHealthChecker_RecordRequest_Success(t *testing.T) {
	config := &PassiveHealthConfig{
		Enabled:              true,
		ConsecutiveFailures:  3,
		IsolationDuration:    30 * time.Second,
		RecoveryInterval:     10 * time.Second,
		ConsecutiveSuccesses: 2,
		FailureStatusCodes:   []int{500, 502, 503},
		TimeoutAsFailure:     true,
	}

	checker := NewPassiveHealthChecker(config, nil)
	
	target := &types.Target{
		Host:    "example.com",
		Port:    80,
		Healthy: true,
	}

	checker.AddTarget("upstream1", target)

	// Record successful request
	result := &RequestResult{
		UpstreamID: "upstream1",
		Target:     target,
		StatusCode: 200,
		Error:      nil,
		Duration:   100 * time.Millisecond,
		IsTimeout:  false,
		Timestamp:  time.Now(),
	}

	checker.RecordRequest(result)

	// Check state
	targetKey := "upstream1:example.com:80"
	checker.mu.RLock()
	state := checker.targets[targetKey]
	checker.mu.RUnlock()

	if state.consecutiveSuccesses != 1 {
		t.Errorf("Expected 1 consecutive success, got %d", state.consecutiveSuccesses)
	}

	if state.consecutiveFailures != 0 {
		t.Errorf("Expected 0 consecutive failures, got %d", state.consecutiveFailures)
	}

	if state.totalSuccesses != 1 {
		t.Errorf("Expected 1 total success, got %d", state.totalSuccesses)
	}

	if !state.healthy {
		t.Error("Target should remain healthy")
	}
}

func TestPassiveHealthChecker_RecordRequest_Failure(t *testing.T) {
	config := &PassiveHealthConfig{
		Enabled:              true,
		ConsecutiveFailures:  3,
		IsolationDuration:    30 * time.Second,
		RecoveryInterval:     10 * time.Second,
		ConsecutiveSuccesses: 2,
		FailureStatusCodes:   []int{500, 502, 503},
		TimeoutAsFailure:     true,
	}

	var callbackCalled bool
	var callbackHealthy bool

	callback := func(upstreamID, targetKey string, healthy bool) {
		callbackCalled = true
		callbackHealthy = healthy
	}

	checker := NewPassiveHealthChecker(config, callback)
	
	target := &types.Target{
		Host:    "example.com",
		Port:    80,
		Healthy: true,
	}

	checker.AddTarget("upstream1", target)

	// Record multiple failed requests to trigger isolation
	for i := 0; i < 3; i++ {
		result := &RequestResult{
			UpstreamID: "upstream1",
			Target:     target,
			StatusCode: 500,
			Error:      nil,
			Duration:   100 * time.Millisecond,
			IsTimeout:  false,
			Timestamp:  time.Now(),
		}
		checker.RecordRequest(result)
	}

	// Check state
	targetKey := "upstream1:example.com:80"
	checker.mu.RLock()
	state := checker.targets[targetKey]
	checker.mu.RUnlock()

	if state.consecutiveFailures != 3 {
		t.Errorf("Expected 3 consecutive failures, got %d", state.consecutiveFailures)
	}

	if state.totalFailures != 3 {
		t.Errorf("Expected 3 total failures, got %d", state.totalFailures)
	}

	if !state.isolated {
		t.Error("Target should be isolated")
	}

	if state.healthy {
		t.Error("Target should be unhealthy")
	}

	if !callbackCalled {
		t.Error("Callback should have been called")
	}

	if callbackHealthy {
		t.Error("Callback should indicate unhealthy")
	}
}

func TestPassiveHealthChecker_IsRequestFailure(t *testing.T) {
	config := &PassiveHealthConfig{
		Enabled:              true,
		ConsecutiveFailures:  3,
		IsolationDuration:    30 * time.Second,
		RecoveryInterval:     10 * time.Second,
		ConsecutiveSuccesses: 2,
		FailureStatusCodes:   []int{500, 502, 503},
		TimeoutAsFailure:     true,
	}

	checker := NewPassiveHealthChecker(config, nil)

	tests := []struct {
		name     string
		result   *RequestResult
		expected bool
	}{
		{
			name: "Success - 200",
			result: &RequestResult{
				StatusCode: 200,
				Error:      nil,
				IsTimeout:  false,
			},
			expected: false,
		},
		{
			name: "Failure - 500",
			result: &RequestResult{
				StatusCode: 500,
				Error:      nil,
				IsTimeout:  false,
			},
			expected: true,
		},
		{
			name: "Failure - Timeout",
			result: &RequestResult{
				StatusCode: 200,
				Error:      nil,
				IsTimeout:  true,
			},
			expected: true,
		},
		{
			name: "Failure - Error",
			result: &RequestResult{
				StatusCode: 200,
				Error:      &testError{},
				IsTimeout:  false,
			},
			expected: true,
		},
		{
			name: "Success - 404 (not in failure codes)",
			result: &RequestResult{
				StatusCode: 404,
				Error:      nil,
				IsTimeout:  false,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.isRequestFailure(tt.result)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestPassiveHealthChecker_IsTargetHealthy(t *testing.T) {
	config := &PassiveHealthConfig{
		Enabled:              true,
		ConsecutiveFailures:  3,
		IsolationDuration:    30 * time.Second,
		RecoveryInterval:     10 * time.Second,
		ConsecutiveSuccesses: 2,
		FailureStatusCodes:   []int{500, 502, 503},
		TimeoutAsFailure:     true,
	}

	checker := NewPassiveHealthChecker(config, nil)
	
	target := &types.Target{
		Host:    "example.com",
		Port:    80,
		Healthy: true,
	}

	// Test unknown target (should be healthy by default)
	healthy := checker.IsTargetHealthy("upstream1", target)
	if !healthy {
		t.Error("Unknown target should be healthy by default")
	}

	// Add target and test
	checker.AddTarget("upstream1", target)
	healthy = checker.IsTargetHealthy("upstream1", target)
	if !healthy {
		t.Error("Newly added target should be healthy")
	}

	// Isolate target and test
	targetKey := "upstream1:example.com:80"
	checker.mu.Lock()
	state := checker.targets[targetKey]
	state.isolated = true
	state.healthy = false
	checker.mu.Unlock()

	healthy = checker.IsTargetHealthy("upstream1", target)
	if healthy {
		t.Error("Isolated target should be unhealthy")
	}
}

// testError is a simple error implementation for testing
type testError struct{}

func (e *testError) Error() string {
	return "test error"
}
