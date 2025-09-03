package circuitbreaker

import (
	"testing"
	"time"
)

func TestCircuitBreakerStates(t *testing.T) {
	config := &Config{
		FailureThreshold:         3,
		RecoveryTimeout:          100 * time.Millisecond,
		RequestVolumeThreshold:   5,
		ErrorPercentageThreshold: 50,
		MaxHalfOpenRequests:      2,
		SuccessThreshold:         2,
	}

	cb := New("test", config)

	// Initial state should be closed
	if cb.GetState() != StateClosed {
		t.Errorf("Expected initial state to be CLOSED, got %s", cb.GetState())
	}

	// Should allow execution in closed state
	if !cb.CanExecute() {
		t.Error("Expected CanExecute to return true in CLOSED state")
	}
}

func TestCircuitBreakerTripping(t *testing.T) {
	config := &Config{
		FailureThreshold:         3,
		RecoveryTimeout:          100 * time.Millisecond,
		RequestVolumeThreshold:   5,
		ErrorPercentageThreshold: 50,
		MaxHalfOpenRequests:      2,
		SuccessThreshold:         2,
	}

	cb := New("test", config)

	// Record failures to trip the circuit (consecutive failures should trip immediately)
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()

	// Should be open due to consecutive failures
	if cb.GetState() != StateOpen {
		t.Errorf("Expected state to be OPEN after consecutive failures, got %s", cb.GetState())
	}

	// Should not allow execution in open state
	if cb.CanExecute() {
		t.Error("Expected CanExecute to return false in OPEN state")
	}
}

func TestCircuitBreakerRecovery(t *testing.T) {
	config := &Config{
		FailureThreshold:         3,
		RecoveryTimeout:          50 * time.Millisecond,
		RequestVolumeThreshold:   5,
		ErrorPercentageThreshold: 50,
		MaxHalfOpenRequests:      2,
		SuccessThreshold:         2,
	}

	cb := New("test", config)

	// Trip the circuit breaker
	for i := 0; i < 8; i++ {
		cb.RecordFailure()
	}

	if cb.GetState() != StateOpen {
		t.Errorf("Expected state to be OPEN, got %s", cb.GetState())
	}

	// Wait for recovery timeout
	time.Sleep(60 * time.Millisecond)

	// Should transition to half-open on next CanExecute call
	if !cb.CanExecute() {
		t.Error("Expected CanExecute to return true after recovery timeout")
	}

	if cb.GetState() != StateHalfOpen {
		t.Errorf("Expected state to be HALF_OPEN after recovery timeout, got %s", cb.GetState())
	}
}

func TestCircuitBreakerHalfOpenBehavior(t *testing.T) {
	config := &Config{
		FailureThreshold:         3,
		RecoveryTimeout:          50 * time.Millisecond,
		RequestVolumeThreshold:   5,
		ErrorPercentageThreshold: 50,
		MaxHalfOpenRequests:      2,
		SuccessThreshold:         2,
	}

	cb := New("test", config)

	// Trip the circuit breaker
	for i := 0; i < 8; i++ {
		cb.RecordFailure()
	}

	// Wait for recovery timeout
	time.Sleep(60 * time.Millisecond)

	// Transition to half-open
	cb.CanExecute()

	// Should allow limited requests in half-open state
	if !cb.CanExecute() {
		t.Error("Expected first request to be allowed in HALF_OPEN state")
	}

	if !cb.CanExecute() {
		t.Error("Expected second request to be allowed in HALF_OPEN state")
	}

	// Third request should be rejected (exceeds MaxHalfOpenRequests)
	if cb.CanExecute() {
		t.Error("Expected third request to be rejected in HALF_OPEN state")
	}
}

func TestCircuitBreakerHalfOpenToClosedTransition(t *testing.T) {
	config := &Config{
		FailureThreshold:         3,
		RecoveryTimeout:          50 * time.Millisecond,
		RequestVolumeThreshold:   5,
		ErrorPercentageThreshold: 50,
		MaxHalfOpenRequests:      3,
		SuccessThreshold:         2,
	}

	cb := New("test", config)

	// Trip the circuit breaker
	for i := 0; i < 8; i++ {
		cb.RecordFailure()
	}

	// Wait for recovery timeout and transition to half-open
	time.Sleep(60 * time.Millisecond)
	cb.CanExecute()

	// Record successful requests to close the circuit
	cb.RecordSuccess()
	cb.RecordSuccess()

	// Should transition back to closed
	if cb.GetState() != StateClosed {
		t.Errorf("Expected state to be CLOSED after successful requests, got %s", cb.GetState())
	}
}

func TestCircuitBreakerHalfOpenToOpenTransition(t *testing.T) {
	config := &Config{
		FailureThreshold:         3,
		RecoveryTimeout:          50 * time.Millisecond,
		RequestVolumeThreshold:   5,
		ErrorPercentageThreshold: 50,
		MaxHalfOpenRequests:      3,
		SuccessThreshold:         2,
	}

	cb := New("test", config)

	// Trip the circuit breaker
	for i := 0; i < 8; i++ {
		cb.RecordFailure()
	}

	// Wait for recovery timeout and transition to half-open
	time.Sleep(60 * time.Millisecond)
	cb.CanExecute()

	// Record a failure in half-open state
	cb.RecordFailure()

	// Should transition back to open
	if cb.GetState() != StateOpen {
		t.Errorf("Expected state to be OPEN after failure in HALF_OPEN, got %s", cb.GetState())
	}
}

func TestCircuitBreakerErrorPercentageThreshold(t *testing.T) {
	config := &Config{
		FailureThreshold:         20, // High threshold to test percentage
		RecoveryTimeout:          100 * time.Millisecond,
		RequestVolumeThreshold:   10,
		ErrorPercentageThreshold: 60, // 60% error rate
		MaxHalfOpenRequests:      2,
		SuccessThreshold:         2,
	}

	cb := New("test", config)

	// Record requests with 70% error rate (7 failures, 3 successes)
	// Mix them to avoid consecutive failure threshold
	cb.RecordFailure()
	cb.RecordSuccess()
	cb.RecordFailure()
	cb.RecordSuccess()
	cb.RecordFailure()
	cb.RecordSuccess()
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()

	// Should trip due to error percentage (7/10 = 70% > 60%)
	if cb.GetState() != StateOpen {
		stats := cb.GetStatistics()
		t.Errorf("Expected state to be OPEN due to error percentage (%.2f%%), got %s", stats.ErrorRate(), cb.GetState())
	}
}

func TestCircuitBreakerStatistics(t *testing.T) {
	config := DefaultConfig()
	cb := New("test", config)

	// Record some requests
	cb.RecordSuccess()
	cb.RecordSuccess()
	cb.RecordFailure()

	stats := cb.GetStatistics()

	if stats.TotalRequests != 3 {
		t.Errorf("Expected TotalRequests to be 3, got %d", stats.TotalRequests)
	}

	if stats.SuccessfulRequests != 2 {
		t.Errorf("Expected SuccessfulRequests to be 2, got %d", stats.SuccessfulRequests)
	}

	if stats.FailedRequests != 1 {
		t.Errorf("Expected FailedRequests to be 1, got %d", stats.FailedRequests)
	}

	expectedErrorRate := float64(1) / float64(3) * 100
	if stats.ErrorRate() != expectedErrorRate {
		t.Errorf("Expected ErrorRate to be %.2f, got %.2f", expectedErrorRate, stats.ErrorRate())
	}
}

func TestCircuitBreakerReset(t *testing.T) {
	config := DefaultConfig()
	cb := New("test", config)

	// Trip the circuit breaker
	for i := 0; i < 10; i++ {
		cb.RecordFailure()
	}

	if cb.GetState() != StateOpen {
		t.Errorf("Expected state to be OPEN, got %s", cb.GetState())
	}

	// Reset the circuit breaker
	cb.Reset()

	if cb.GetState() != StateClosed {
		t.Errorf("Expected state to be CLOSED after reset, got %s", cb.GetState())
	}

	stats := cb.GetStatistics()
	if stats.TotalRequests != 0 {
		t.Errorf("Expected TotalRequests to be 0 after reset, got %d", stats.TotalRequests)
	}
}

func TestCircuitBreakerStateChangeCallback(t *testing.T) {
	config := DefaultConfig()
	cb := New("test", config)

	callbackCh := make(chan struct {
		called bool
		fromState State
		toState State
	}, 1)

	cb.SetStateChangeCallback(func(name string, from, to State) {
		callbackCh <- struct {
			called bool
			fromState State
			toState State
		}{
			called: true,
			fromState: from,
			toState: to,
		}
	})

	// Trip the circuit breaker
	for i := 0; i < 10; i++ {
		cb.RecordFailure()
	}

	// Wait for callback
	select {
	case result := <-callbackCh:
		if !result.called {
			t.Error("Expected state change callback to be called")
		}

		if result.fromState != StateClosed {
			t.Errorf("Expected fromState to be CLOSED, got %s", result.fromState)
		}

		if result.toState != StateOpen {
			t.Errorf("Expected toState to be OPEN, got %s", result.toState)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for state change callback")
	}
}
