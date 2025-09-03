package health

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
)

// Checker represents the health checker
type Checker struct {
	config    *config.Config
	mu        sync.RWMutex
	checks    map[string]*healthCheck
	stopCh    chan struct{}
	wg        sync.WaitGroup
	callbacks map[string]HealthCallback
}

// HealthCallback is called when health status changes
type HealthCallback func(upstreamID, targetKey string, healthy bool)

// healthCheck represents a single health check
type healthCheck struct {
	upstreamID         string
	targetKey          string
	target             *Target
	config             *HealthCheckConfig
	healthy            bool
	consecutiveSuccess int
	consecutiveFailure int
	lastCheck          time.Time
	lastError          error
}

// Target represents a backend target
type Target struct {
	Host    string `json:"host"`
	Port    int    `json:"port"`
	Weight  int    `json:"weight"`
	Healthy bool   `json:"healthy"`
}

// HealthCheckConfig represents health check configuration
type HealthCheckConfig struct {
	Type               string `json:"type"`
	Path               string `json:"path"`
	Interval           int    `json:"interval"`
	Timeout            int    `json:"timeout"`
	HealthyThreshold   int    `json:"healthy_threshold"`
	UnhealthyThreshold int    `json:"unhealthy_threshold"`
}

// NewChecker creates a new health checker
func NewChecker(cfg *config.Config) *Checker {
	return &Checker{
		config:    cfg,
		checks:    make(map[string]*healthCheck),
		stopCh:    make(chan struct{}),
		callbacks: make(map[string]HealthCallback),
	}
}

// Start starts the health checker
func (hc *Checker) Start() error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	// Start health check goroutines for existing checks
	for _, check := range hc.checks {
		hc.wg.Add(1)
		go hc.runHealthCheck(check)
	}

	return nil
}

// Stop stops the health checker
func (hc *Checker) Stop() error {
	close(hc.stopCh)
	hc.wg.Wait()
	return nil
}

// AddTarget adds a target for health checking
func (hc *Checker) AddTarget(upstreamID string, target *Target, checkConfig *HealthCheckConfig) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	targetKey := fmt.Sprintf("%s:%d", target.Host, target.Port)
	checkKey := fmt.Sprintf("%s:%s", upstreamID, targetKey)

	// Create health check
	check := &healthCheck{
		upstreamID:         upstreamID,
		targetKey:          targetKey,
		target:             target,
		config:             checkConfig,
		healthy:            true, // Assume healthy initially
		consecutiveSuccess: 0,
		consecutiveFailure: 0,
		lastCheck:          time.Now(),
	}

	hc.checks[checkKey] = check

	// Start health check goroutine
	hc.wg.Add(1)
	go hc.runHealthCheck(check)

	return nil
}

// RemoveTarget removes a target from health checking
func (hc *Checker) RemoveTarget(upstreamID string, targetKey string) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	checkKey := fmt.Sprintf("%s:%s", upstreamID, targetKey)
	delete(hc.checks, checkKey)

	return nil
}

// GetTargetHealth returns the health status of a target
func (hc *Checker) GetTargetHealth(upstreamID string, targetKey string) (bool, error) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	checkKey := fmt.Sprintf("%s:%s", upstreamID, targetKey)
	check, exists := hc.checks[checkKey]
	if !exists {
		return false, fmt.Errorf("health check not found for %s", checkKey)
	}

	return check.healthy, nil
}

// GetUpstreamHealth returns the health status of all targets in an upstream
func (hc *Checker) GetUpstreamHealth(upstreamID string) (map[string]bool, error) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	health := make(map[string]bool)
	for _, check := range hc.checks {
		if check.upstreamID == upstreamID {
			health[check.targetKey] = check.healthy
		}
	}

	return health, nil
}

// RegisterCallback registers a callback for health status changes
func (hc *Checker) RegisterCallback(name string, callback HealthCallback) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	hc.callbacks[name] = callback
}

// UnregisterCallback unregisters a callback
func (hc *Checker) UnregisterCallback(name string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	delete(hc.callbacks, name)
}

// runHealthCheck runs health checks for a target
func (hc *Checker) runHealthCheck(check *healthCheck) {
	defer hc.wg.Done()

	interval := time.Duration(check.config.Interval) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-hc.stopCh:
			return
		case <-ticker.C:
			hc.performHealthCheck(check)
		}
	}
}

// performHealthCheck performs a single health check
func (hc *Checker) performHealthCheck(check *healthCheck) {
	var healthy bool
	var err error

	switch check.config.Type {
	case "http":
		healthy, err = hc.performHTTPCheck(check)
	case "tcp":
		healthy, err = hc.performTCPCheck(check)
	default:
		healthy, err = hc.performHTTPCheck(check) // Default to HTTP
	}

	hc.mu.Lock()
	defer hc.mu.Unlock()

	check.lastCheck = time.Now()
	check.lastError = err

	// Update consecutive counters
	if healthy {
		check.consecutiveSuccess++
		check.consecutiveFailure = 0
	} else {
		check.consecutiveFailure++
		check.consecutiveSuccess = 0
	}

	// Determine new health status
	newHealthy := check.healthy

	if !check.healthy && check.consecutiveSuccess >= check.config.HealthyThreshold {
		// Target becomes healthy
		newHealthy = true
	} else if check.healthy && check.consecutiveFailure >= check.config.UnhealthyThreshold {
		// Target becomes unhealthy
		newHealthy = false
	}

	// Update health status and notify callbacks if changed
	if newHealthy != check.healthy {
		check.healthy = newHealthy
		hc.notifyCallbacks(check.upstreamID, check.targetKey, newHealthy)
	}
}

// performHTTPCheck performs an HTTP health check
func (hc *Checker) performHTTPCheck(check *healthCheck) (bool, error) {
	timeout := time.Duration(check.config.Timeout) * time.Second
	client := &http.Client{
		Timeout: timeout,
	}

	url := fmt.Sprintf("http://%s:%d%s", check.target.Host, check.target.Port, check.config.Path)
	
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Consider 2xx and 3xx status codes as healthy
	return resp.StatusCode >= 200 && resp.StatusCode < 400, nil
}

// performTCPCheck performs a TCP health check
func (hc *Checker) performTCPCheck(check *healthCheck) (bool, error) {
	timeout := time.Duration(check.config.Timeout) * time.Second
	address := fmt.Sprintf("%s:%d", check.target.Host, check.target.Port)

	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return false, fmt.Errorf("TCP connection failed: %w", err)
	}
	defer conn.Close()

	return true, nil
}

// notifyCallbacks notifies all registered callbacks
func (hc *Checker) notifyCallbacks(upstreamID, targetKey string, healthy bool) {
	for _, callback := range hc.callbacks {
		go callback(upstreamID, targetKey, healthy)
	}
}

// Health returns the health status of the health checker
func (hc *Checker) Health() map[string]interface{} {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	totalChecks := len(hc.checks)
	healthyChecks := 0
	checkDetails := make(map[string]interface{})

	for checkKey, check := range hc.checks {
		if check.healthy {
			healthyChecks++
		}

		checkDetails[checkKey] = map[string]interface{}{
			"healthy":             check.healthy,
			"consecutive_success": check.consecutiveSuccess,
			"consecutive_failure": check.consecutiveFailure,
			"last_check":          check.lastCheck.Unix(),
			"last_error":          getErrorString(check.lastError),
		}
	}

	return map[string]interface{}{
		"status":        "healthy",
		"total_checks":  totalChecks,
		"healthy_checks": healthyChecks,
		"checks":        checkDetails,
	}
}

// Metrics returns health checker metrics
func (hc *Checker) Metrics() map[string]interface{} {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	totalChecks := len(hc.checks)
	healthyChecks := 0

	for _, check := range hc.checks {
		if check.healthy {
			healthyChecks++
		}
	}

	return map[string]interface{}{
		"total_checks":   totalChecks,
		"healthy_checks": healthyChecks,
		"unhealthy_checks": totalChecks - healthyChecks,
	}
}

// getErrorString safely converts error to string
func getErrorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
