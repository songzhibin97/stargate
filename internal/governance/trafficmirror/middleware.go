package trafficmirror

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
)

// Middleware represents the traffic mirroring middleware
type Middleware struct {
	config      *config.TrafficMirrorConfig
	mirrors     map[string]*MirrorTarget
	mutex       sync.RWMutex
	client      *http.Client
	requestPool sync.Pool
}

// MirrorTarget represents a mirror destination
type MirrorTarget struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	URL         string            `json:"url"`
	SampleRate  float64           `json:"sample_rate"`  // 0.0 - 1.0
	Timeout     time.Duration     `json:"timeout"`
	Headers     map[string]string `json:"headers"`
	Enabled     bool              `json:"enabled"`
	Metadata    map[string]string `json:"metadata"`
	
	// Statistics
	TotalRequests   int64     `json:"total_requests"`
	MirroredRequests int64    `json:"mirrored_requests"`
	FailedRequests  int64     `json:"failed_requests"`
	LastMirrorTime  time.Time `json:"last_mirror_time"`
}

// MirrorRequest represents a request to be mirrored
type MirrorRequest struct {
	OriginalRequest *http.Request
	Body            []byte
	Target          *MirrorTarget
	RouteID         string
	Timestamp       time.Time
}

// NewMiddleware creates a new traffic mirror middleware
func NewMiddleware(config *config.TrafficMirrorConfig) (*Middleware, error) {
	if config == nil {
		return nil, fmt.Errorf("traffic mirror config cannot be nil")
	}

	// Create HTTP client for mirror requests
	client := &http.Client{
		Timeout: 30 * time.Second, // Default timeout
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	middleware := &Middleware{
		config:  config,
		mirrors: make(map[string]*MirrorTarget),
		client:  client,
		requestPool: sync.Pool{
			New: func() interface{} {
				return &MirrorRequest{}
			},
		},
	}

	// Initialize mirror targets from config
	for _, mirrorConfig := range config.Mirrors {
		target := &MirrorTarget{
			ID:         mirrorConfig.ID,
			Name:       mirrorConfig.Name,
			URL:        mirrorConfig.URL,
			SampleRate: mirrorConfig.SampleRate,
			Timeout:    mirrorConfig.Timeout,
			Headers:    mirrorConfig.Headers,
			Enabled:    mirrorConfig.Enabled,
			Metadata:   mirrorConfig.Metadata,
		}
		middleware.mirrors[target.ID] = target
	}

	return middleware, nil
}

// Handler returns the HTTP middleware handler
func (m *Middleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip if traffic mirroring is disabled
			if !m.config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Create response wrapper to capture response details
			wrapper := &responseWrapper{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				body:          &bytes.Buffer{},
			}

			// Read and buffer request body for mirroring
			var requestBody []byte
			if r.Body != nil {
				var err error
				requestBody, err = io.ReadAll(r.Body)
				if err != nil {
					log.Printf("Failed to read request body for mirroring: %v", err)
					next.ServeHTTP(w, r)
					return
				}
				// Restore request body for the main request
				r.Body = io.NopCloser(bytes.NewReader(requestBody))
			}

			// Process the main request
			next.ServeHTTP(wrapper, r)

			// After main request is processed, mirror it asynchronously
			m.mirrorRequestAsync(r, requestBody, wrapper.statusCode)
		})
	}
}

// mirrorRequestAsync mirrors the request asynchronously to configured targets
func (m *Middleware) mirrorRequestAsync(originalReq *http.Request, body []byte, responseStatus int) {
	// Get route ID from context
	routeID := m.getRouteID(originalReq)

	// Find applicable mirror targets for this route
	targets := m.getApplicableMirrors(routeID, originalReq)
	if len(targets) == 0 {
		return
	}

	// Mirror to each applicable target in separate goroutines
	for _, target := range targets {
		go m.mirrorToTarget(originalReq, body, target, routeID, responseStatus)
	}
}

// mirrorToTarget mirrors the request to a specific target
func (m *Middleware) mirrorToTarget(originalReq *http.Request, body []byte, target *MirrorTarget, routeID string, responseStatus int) {
	// Update statistics
	m.mutex.Lock()
	target.TotalRequests++
	m.mutex.Unlock()

	// Check if we should mirror this request based on sample rate
	if !m.shouldMirror(target.SampleRate) {
		return
	}

	// Update mirrored request count
	m.mutex.Lock()
	target.MirroredRequests++
	target.LastMirrorTime = time.Now()
	m.mutex.Unlock()

	// Create mirror request
	mirrorReq, err := m.createMirrorRequest(originalReq, body, target)
	if err != nil {
		log.Printf("Failed to create mirror request for target %s: %v", target.ID, err)
		m.recordFailure(target)
		return
	}

	// Set timeout for mirror request
	ctx, cancel := context.WithTimeout(context.Background(), target.Timeout)
	defer cancel()
	mirrorReq = mirrorReq.WithContext(ctx)

	// Send mirror request
	resp, err := m.client.Do(mirrorReq)
	if err != nil {
		log.Printf("Failed to send mirror request to target %s: %v", target.ID, err)
		m.recordFailure(target)
		return
	}
	defer resp.Body.Close()

	// Log mirror request result (optional)
	if m.config.LogMirrorRequests {
		log.Printf("Mirror request sent to %s: %s %s -> %d", 
			target.Name, originalReq.Method, originalReq.URL.Path, resp.StatusCode)
	}
}

// createMirrorRequest creates a new HTTP request for mirroring
func (m *Middleware) createMirrorRequest(originalReq *http.Request, body []byte, target *MirrorTarget) (*http.Request, error) {
	// Create new request with mirror target URL
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}

	mirrorReq, err := http.NewRequest(originalReq.Method, target.URL+originalReq.URL.Path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create mirror request: %w", err)
	}

	// Copy query parameters
	mirrorReq.URL.RawQuery = originalReq.URL.RawQuery

	// Copy headers from original request
	for key, values := range originalReq.Header {
		// Skip hop-by-hop headers
		if m.isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			mirrorReq.Header.Add(key, value)
		}
	}

	// Add custom headers from mirror target configuration
	for key, value := range target.Headers {
		mirrorReq.Header.Set(key, value)
	}

	// Add mirror identification headers
	mirrorReq.Header.Set("X-Mirror-Source", "stargate")
	mirrorReq.Header.Set("X-Mirror-Target", target.ID)
	mirrorReq.Header.Set("X-Mirror-Timestamp", time.Now().Format(time.RFC3339))

	return mirrorReq, nil
}

// getApplicableMirrors returns mirror targets applicable to the given route
func (m *Middleware) getApplicableMirrors(routeID string, req *http.Request) []*MirrorTarget {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var targets []*MirrorTarget
	for _, target := range m.mirrors {
		if !target.Enabled {
			continue
		}

		// Check if target applies to this route
		if m.targetAppliesTo(target, routeID, req) {
			targets = append(targets, target)
		}
	}

	return targets
}

// targetAppliesTo checks if a mirror target applies to the given route/request
func (m *Middleware) targetAppliesTo(target *MirrorTarget, routeID string, req *http.Request) bool {
	// If target has route-specific configuration, check it
	if routeFilter, exists := target.Metadata["route_filter"]; exists {
		if routeFilter != routeID && routeFilter != "*" {
			return false
		}
	}

	// If target has method filter, check it
	if methodFilter, exists := target.Metadata["method_filter"]; exists {
		if methodFilter != req.Method && methodFilter != "*" {
			return false
		}
	}

	// If target has path filter, check it
	if pathFilter, exists := target.Metadata["path_filter"]; exists {
		if pathFilter != req.URL.Path && pathFilter != "*" {
			return false
		}
	}

	return true
}

// shouldMirror determines if a request should be mirrored based on sample rate
func (m *Middleware) shouldMirror(sampleRate float64) bool {
	if sampleRate <= 0 {
		return false
	}
	if sampleRate >= 1.0 {
		return true
	}

	// Use a simple random sampling
	// In production, you might want to use more sophisticated sampling
	return m.generateRandomFloat() < sampleRate
}

// generateRandomFloat generates a random float between 0 and 1
func (m *Middleware) generateRandomFloat() float64 {
	// Simple random number generation for sampling
	// In production, consider using crypto/rand for better randomness
	return float64(time.Now().UnixNano()%10000) / 10000.0
}

// getRouteID extracts route ID from request context
func (m *Middleware) getRouteID(req *http.Request) string {
	if routeID := req.Context().Value("route_id"); routeID != nil {
		if id, ok := routeID.(string); ok {
			return id
		}
	}
	return "default"
}

// recordFailure records a failed mirror request
func (m *Middleware) recordFailure(target *MirrorTarget) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	target.FailedRequests++
}

// isHopByHopHeader checks if a header is hop-by-hop
func (m *Middleware) isHopByHopHeader(header string) bool {
	hopByHopHeaders := []string{
		"Connection", "Keep-Alive", "Proxy-Authenticate",
		"Proxy-Authorization", "Te", "Trailers", "Transfer-Encoding", "Upgrade",
	}
	
	for _, h := range hopByHopHeaders {
		if header == h {
			return true
		}
	}
	return false
}

// responseWrapper wraps http.ResponseWriter to capture response details
type responseWrapper struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

// WriteHeader captures the status code
func (rw *responseWrapper) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the response body and writes to the original writer
func (rw *responseWrapper) Write(data []byte) (int, error) {
	if rw.body != nil {
		rw.body.Write(data)
	}
	return rw.ResponseWriter.Write(data)
}

// AddMirrorTarget adds a new mirror target
func (m *Middleware) AddMirrorTarget(target *MirrorTarget) error {
	if target == nil {
		return fmt.Errorf("mirror target cannot be nil")
	}

	if target.ID == "" {
		return fmt.Errorf("mirror target ID cannot be empty")
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.mirrors[target.ID] = target
	return nil
}

// RemoveMirrorTarget removes a mirror target
func (m *Middleware) RemoveMirrorTarget(targetID string) error {
	if targetID == "" {
		return fmt.Errorf("target ID cannot be empty")
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.mirrors[targetID]; !exists {
		return fmt.Errorf("mirror target %s not found", targetID)
	}

	delete(m.mirrors, targetID)
	return nil
}

// UpdateMirrorTarget updates an existing mirror target
func (m *Middleware) UpdateMirrorTarget(target *MirrorTarget) error {
	if target == nil {
		return fmt.Errorf("mirror target cannot be nil")
	}

	if target.ID == "" {
		return fmt.Errorf("mirror target ID cannot be empty")
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.mirrors[target.ID]; !exists {
		return fmt.Errorf("mirror target %s not found", target.ID)
	}

	m.mirrors[target.ID] = target
	return nil
}

// GetMirrorTarget retrieves a mirror target by ID
func (m *Middleware) GetMirrorTarget(targetID string) (*MirrorTarget, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	target, exists := m.mirrors[targetID]
	if !exists {
		return nil, fmt.Errorf("mirror target %s not found", targetID)
	}

	// Return a copy to prevent external modification
	targetCopy := *target
	return &targetCopy, nil
}

// ListMirrorTargets returns all mirror targets
func (m *Middleware) ListMirrorTargets() []*MirrorTarget {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	targets := make([]*MirrorTarget, 0, len(m.mirrors))
	for _, target := range m.mirrors {
		// Return copies to prevent external modification
		targetCopy := *target
		targets = append(targets, &targetCopy)
	}

	return targets
}

// EnableMirrorTarget enables a mirror target
func (m *Middleware) EnableMirrorTarget(targetID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	target, exists := m.mirrors[targetID]
	if !exists {
		return fmt.Errorf("mirror target %s not found", targetID)
	}

	target.Enabled = true
	return nil
}

// DisableMirrorTarget disables a mirror target
func (m *Middleware) DisableMirrorTarget(targetID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	target, exists := m.mirrors[targetID]
	if !exists {
		return fmt.Errorf("mirror target %s not found", targetID)
	}

	target.Enabled = false
	return nil
}

// GetStatistics returns statistics for all mirror targets
func (m *Middleware) GetStatistics() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	stats := map[string]interface{}{
		"enabled":       m.config.Enabled,
		"targets_count": len(m.mirrors),
		"timestamp":     time.Now().Unix(),
	}

	targets := make(map[string]interface{})
	totalRequests := int64(0)
	totalMirrored := int64(0)
	totalFailed := int64(0)

	for id, target := range m.mirrors {
		targets[id] = map[string]interface{}{
			"name":             target.Name,
			"url":              target.URL,
			"enabled":          target.Enabled,
			"sample_rate":      target.SampleRate,
			"total_requests":   target.TotalRequests,
			"mirrored_requests": target.MirroredRequests,
			"failed_requests":  target.FailedRequests,
			"last_mirror_time": target.LastMirrorTime,
			"success_rate":     m.calculateSuccessRate(target),
		}

		totalRequests += target.TotalRequests
		totalMirrored += target.MirroredRequests
		totalFailed += target.FailedRequests
	}

	stats["targets"] = targets
	stats["total_requests"] = totalRequests
	stats["total_mirrored"] = totalMirrored
	stats["total_failed"] = totalFailed
	stats["overall_success_rate"] = m.calculateOverallSuccessRate(totalMirrored, totalFailed)

	return stats
}

// calculateSuccessRate calculates success rate for a target
func (m *Middleware) calculateSuccessRate(target *MirrorTarget) float64 {
	if target.MirroredRequests == 0 {
		return 0.0
	}
	successfulRequests := target.MirroredRequests - target.FailedRequests
	return float64(successfulRequests) / float64(target.MirroredRequests) * 100
}

// calculateOverallSuccessRate calculates overall success rate
func (m *Middleware) calculateOverallSuccessRate(mirrored, failed int64) float64 {
	if mirrored == 0 {
		return 0.0
	}
	successful := mirrored - failed
	return float64(successful) / float64(mirrored) * 100
}

// ResetStatistics resets statistics for all mirror targets
func (m *Middleware) ResetStatistics() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for _, target := range m.mirrors {
		target.TotalRequests = 0
		target.MirroredRequests = 0
		target.FailedRequests = 0
		target.LastMirrorTime = time.Time{}
	}
}

// HealthCheck returns the health status of the traffic mirror middleware
func (m *Middleware) HealthCheck() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	health := map[string]interface{}{
		"status":    "healthy",
		"enabled":   m.config.Enabled,
		"timestamp": time.Now().Unix(),
	}

	// Check health of each mirror target
	targetHealth := make(map[string]interface{})
	healthyTargets := 0
	totalTargets := 0

	for id, target := range m.mirrors {
		isHealthy := target.Enabled && m.calculateSuccessRate(target) >= 50.0 // 50% success rate threshold
		if target.MirroredRequests == 0 {
			isHealthy = target.Enabled // Consider enabled targets with no requests as healthy
		}

		targetHealth[id] = map[string]interface{}{
			"healthy":      isHealthy,
			"enabled":      target.Enabled,
			"success_rate": m.calculateSuccessRate(target),
		}

		totalTargets++
		if isHealthy {
			healthyTargets++
		}
	}

	health["targets"] = targetHealth
	health["healthy_targets"] = healthyTargets
	health["total_targets"] = totalTargets

	// Overall health status
	if totalTargets == 0 {
		health["status"] = "no_targets"
	} else if healthyTargets == 0 {
		health["status"] = "unhealthy"
	} else if healthyTargets < totalTargets {
		health["status"] = "degraded"
	}

	return health
}

// Close closes the middleware and cleans up resources
func (m *Middleware) Close() error {
	if m.client != nil {
		m.client.CloseIdleConnections()
	}
	return nil
}
