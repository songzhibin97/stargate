package circuitbreaker

import (
	"sync"
	"time"
)

// State represents the circuit breaker state
type State int

const (
	// StateClosed - circuit breaker is closed, requests pass through
	StateClosed State = iota
	// StateOpen - circuit breaker is open, requests fail fast
	StateOpen
	// StateHalfOpen - circuit breaker is half-open, allowing limited requests to test recovery
	StateHalfOpen
)

// String returns the string representation of the state
func (s State) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// Config represents circuit breaker configuration
type Config struct {
	// FailureThreshold is the number of failures that will trip the circuit breaker
	FailureThreshold int `yaml:"failure_threshold"`
	
	// RecoveryTimeout is the duration after which the circuit breaker will attempt recovery
	RecoveryTimeout time.Duration `yaml:"recovery_timeout"`
	
	// RequestVolumeThreshold is the minimum number of requests needed before circuit breaker can trip
	RequestVolumeThreshold int `yaml:"request_volume_threshold"`
	
	// ErrorPercentageThreshold is the error percentage that will trip the circuit breaker
	ErrorPercentageThreshold int `yaml:"error_percentage_threshold"`
	
	// MaxHalfOpenRequests is the maximum number of requests allowed in half-open state
	MaxHalfOpenRequests int `yaml:"max_half_open_requests"`
	
	// SuccessThreshold is the number of consecutive successes needed to close the circuit
	SuccessThreshold int `yaml:"success_threshold"`
}

// DefaultConfig returns a default circuit breaker configuration
func DefaultConfig() *Config {
	return &Config{
		FailureThreshold:         5,
		RecoveryTimeout:          30 * time.Second,
		RequestVolumeThreshold:   10,
		ErrorPercentageThreshold: 50,
		MaxHalfOpenRequests:      3,
		SuccessThreshold:         2,
	}
}

// Statistics holds circuit breaker statistics
type Statistics struct {
	TotalRequests     int64     `json:"total_requests"`
	SuccessfulRequests int64    `json:"successful_requests"`
	FailedRequests    int64     `json:"failed_requests"`
	ConsecutiveFailures int64   `json:"consecutive_failures"`
	ConsecutiveSuccesses int64  `json:"consecutive_successes"`
	LastFailureTime   time.Time `json:"last_failure_time"`
	LastSuccessTime   time.Time `json:"last_success_time"`
	StateChangedAt    time.Time `json:"state_changed_at"`
}

// Reset resets the statistics
func (s *Statistics) Reset() {
	s.TotalRequests = 0
	s.SuccessfulRequests = 0
	s.FailedRequests = 0
	s.ConsecutiveFailures = 0
	s.ConsecutiveSuccesses = 0
	s.LastFailureTime = time.Time{}
	s.LastSuccessTime = time.Time{}
}

// ErrorRate returns the current error rate as a percentage
func (s *Statistics) ErrorRate() float64 {
	if s.TotalRequests == 0 {
		return 0
	}
	return float64(s.FailedRequests) / float64(s.TotalRequests) * 100
}

// CircuitBreaker represents a circuit breaker instance
type CircuitBreaker struct {
	name   string
	config *Config
	state  State
	stats  *Statistics
	mutex  sync.RWMutex
	
	// halfOpenRequests tracks the number of requests in half-open state
	halfOpenRequests int64
	
	// onStateChange callback function called when state changes
	onStateChange func(name string, from, to State)
}

// New creates a new circuit breaker instance
func New(name string, config *Config) *CircuitBreaker {
	if config == nil {
		config = DefaultConfig()
	}
	
	return &CircuitBreaker{
		name:   name,
		config: config,
		state:  StateClosed,
		stats: &Statistics{
			StateChangedAt: time.Now(),
		},
	}
}

// SetStateChangeCallback sets the callback function for state changes
func (cb *CircuitBreaker) SetStateChangeCallback(callback func(name string, from, to State)) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	cb.onStateChange = callback
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() State {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// GetStatistics returns a copy of the current statistics
func (cb *CircuitBreaker) GetStatistics() Statistics {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return *cb.stats
}

// CanExecute determines if a request can be executed based on the current state
func (cb *CircuitBreaker) CanExecute() bool {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if recovery timeout has passed
		if time.Since(cb.stats.StateChangedAt) >= cb.config.RecoveryTimeout {
			cb.changeState(StateHalfOpen)
			cb.halfOpenRequests = 0
			return true
		}
		return false
	case StateHalfOpen:
		// Allow limited requests in half-open state
		if cb.halfOpenRequests < int64(cb.config.MaxHalfOpenRequests) {
			cb.halfOpenRequests++
			return true
		}
		return false
	default:
		return false
	}
}

// RecordSuccess records a successful request
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	cb.stats.TotalRequests++
	cb.stats.SuccessfulRequests++
	cb.stats.ConsecutiveSuccesses++
	cb.stats.ConsecutiveFailures = 0
	cb.stats.LastSuccessTime = time.Now()
	
	// State transition logic for success
	switch cb.state {
	case StateHalfOpen:
		if cb.stats.ConsecutiveSuccesses >= int64(cb.config.SuccessThreshold) {
			cb.changeState(StateClosed)
			cb.stats.Reset()
			cb.stats.StateChangedAt = time.Now()
		}
	}
}

// RecordFailure records a failed request
func (cb *CircuitBreaker) RecordFailure() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	cb.stats.TotalRequests++
	cb.stats.FailedRequests++
	cb.stats.ConsecutiveFailures++
	cb.stats.ConsecutiveSuccesses = 0
	cb.stats.LastFailureTime = time.Now()
	
	// State transition logic for failure
	switch cb.state {
	case StateClosed:
		if cb.shouldTrip() {
			cb.changeState(StateOpen)
		}
	case StateHalfOpen:
		// Any failure in half-open state should open the circuit
		cb.changeState(StateOpen)
		cb.halfOpenRequests = 0
	}
}

// shouldTrip determines if the circuit breaker should trip to open state
func (cb *CircuitBreaker) shouldTrip() bool {
	// Check consecutive failures threshold first (doesn't require volume threshold)
	if cb.stats.ConsecutiveFailures >= int64(cb.config.FailureThreshold) {
		return true
	}

	// Check if we have enough requests to make a percentage-based decision
	if cb.stats.TotalRequests >= int64(cb.config.RequestVolumeThreshold) {
		// Check error percentage threshold
		if cb.stats.ErrorRate() >= float64(cb.config.ErrorPercentageThreshold) {
			return true
		}
	}

	return false
}

// changeState changes the circuit breaker state and triggers callback
func (cb *CircuitBreaker) changeState(newState State) {
	oldState := cb.state
	cb.state = newState
	cb.stats.StateChangedAt = time.Now()
	
	// Trigger callback if set
	if cb.onStateChange != nil {
		go cb.onStateChange(cb.name, oldState, newState)
	}
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	oldState := cb.state
	cb.state = StateClosed
	cb.stats.Reset()
	cb.stats.StateChangedAt = time.Now()
	cb.halfOpenRequests = 0
	
	// Trigger callback if set
	if cb.onStateChange != nil {
		go cb.onStateChange(cb.name, oldState, StateClosed)
	}
}

// GetName returns the circuit breaker name
func (cb *CircuitBreaker) GetName() string {
	return cb.name
}

// GetConfig returns the circuit breaker configuration
func (cb *CircuitBreaker) GetConfig() *Config {
	return cb.config
}
