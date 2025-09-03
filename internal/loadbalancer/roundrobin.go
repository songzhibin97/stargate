package loadbalancer

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/health"
	"github.com/songzhibin97/stargate/internal/types"
)

// RoundRobinBalancer implements round-robin load balancing
type RoundRobinBalancer struct {
	config        *config.Config
	mu            sync.RWMutex
	upstreams     map[string]*upstreamState
	healthChecker *health.ActiveHealthChecker
}

// upstreamState maintains state for an upstream
type upstreamState struct {
	upstream *types.Upstream
	counter  uint64
	targets  []*types.Target
}

// NewRoundRobinBalancer creates a new round-robin load balancer
func NewRoundRobinBalancer(cfg *config.Config) *RoundRobinBalancer {
	rb := &RoundRobinBalancer{
		config:        cfg,
		upstreams:     make(map[string]*upstreamState),
		healthChecker: health.NewActiveHealthChecker(cfg),
	}

	// 添加健康状态变化回调
	rb.healthChecker.AddHealthChangeCallback(rb.onHealthChange)

	// 启动健康检查器
	rb.healthChecker.Start()

	return rb
}

// onHealthChange 健康状态变化回调
func (rb *RoundRobinBalancer) onHealthChange(upstreamID string, target *types.Target, healthy bool) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	// 更新目标实例的健康状态
	if state, exists := rb.upstreams[upstreamID]; exists {
		for _, t := range state.targets {
			if t.Host == target.Host && t.Port == target.Port {
				t.Healthy = healthy
				break
			}
		}
	}
}

// Select selects a target from the upstream using round-robin algorithm
func (rb *RoundRobinBalancer) Select(upstream *types.Upstream) (*types.Target, error) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	state, exists := rb.upstreams[upstream.ID]
	if !exists {
		return nil, fmt.Errorf("upstream %s not found", upstream.ID)
	}

	// Get healthy targets (including passive health check status)
	healthyTargets := rb.getHealthyTargets(state.targets)
	if len(healthyTargets) == 0 {
		return nil, fmt.Errorf("no healthy targets available for upstream %s", upstream.ID)
	}

	// Round-robin selection
	counter := atomic.AddUint64(&state.counter, 1)
	index := (counter - 1) % uint64(len(healthyTargets))
	
	return healthyTargets[index], nil
}

// UpdateUpstream updates or adds an upstream
func (rb *RoundRobinBalancer) UpdateUpstream(upstream *types.Upstream) error {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	// Validate upstream
	if err := rb.validateUpstream(upstream); err != nil {
		return fmt.Errorf("invalid upstream: %w", err)
	}

	// Create or update upstream state
	state := &upstreamState{
		upstream: upstream,
		counter:  0,
		targets:  make([]*types.Target, len(upstream.Targets)),
	}

	// Copy targets
	copy(state.targets, upstream.Targets)

	// Initialize target health status
	for _, target := range state.targets {
		if target.Weight <= 0 {
			target.Weight = 1 // Default weight
		}
		// Keep original health status, don't override
	}

	rb.upstreams[upstream.ID] = state

	// 添加到健康检查器
	if rb.healthChecker != nil {
		rb.healthChecker.AddUpstream(upstream)
	}

	return nil
}

// RemoveUpstream removes an upstream
func (rb *RoundRobinBalancer) RemoveUpstream(id string) error {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if _, exists := rb.upstreams[id]; !exists {
		return fmt.Errorf("upstream %s not found", id)
	}

	delete(rb.upstreams, id)

	// 从健康检查器中移除
	if rb.healthChecker != nil {
		rb.healthChecker.RemoveUpstream(id)
	}

	return nil
}

// GetUpstream returns an upstream by ID
func (rb *RoundRobinBalancer) GetUpstream(id string) (*types.Upstream, error) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	state, exists := rb.upstreams[id]
	if !exists {
		return nil, fmt.Errorf("upstream %s not found", id)
	}

	return state.upstream, nil
}

// ListUpstreams returns all upstreams
func (rb *RoundRobinBalancer) ListUpstreams() []*types.Upstream {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	upstreams := make([]*types.Upstream, 0, len(rb.upstreams))
	for _, state := range rb.upstreams {
		upstreams = append(upstreams, state.upstream)
	}

	return upstreams
}

// UpdateTargetHealth updates the health status of a target
func (rb *RoundRobinBalancer) UpdateTargetHealth(upstreamID string, targetHost string, targetPort int, healthy bool) error {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	state, exists := rb.upstreams[upstreamID]
	if !exists {
		return fmt.Errorf("upstream %s not found", upstreamID)
	}

	// Find and update target
	for _, target := range state.targets {
		if target.Host == targetHost && target.Port == targetPort {
			target.Healthy = healthy
			return nil
		}
	}

	return fmt.Errorf("target %s:%d not found in upstream %s", targetHost, targetPort, upstreamID)
}

// GetTargetHealth returns the health status of all targets in an upstream
func (rb *RoundRobinBalancer) GetTargetHealth(upstreamID string) (map[string]bool, error) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	state, exists := rb.upstreams[upstreamID]
	if !exists {
		return nil, fmt.Errorf("upstream %s not found", upstreamID)
	}

	health := make(map[string]bool)
	for _, target := range state.targets {
		key := fmt.Sprintf("%s:%d", target.Host, target.Port)
		health[key] = target.Healthy
	}

	return health, nil
}

// Health returns the health status of the load balancer
func (rb *RoundRobinBalancer) Health() map[string]interface{} {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	upstreamHealth := make(map[string]interface{})
	totalTargets := 0
	healthyTargets := 0

	for id, state := range rb.upstreams {
		healthy := rb.getHealthyTargets(state.targets)
		upstreamHealth[id] = map[string]interface{}{
			"total_targets":   len(state.targets),
			"healthy_targets": len(healthy),
			"algorithm":       state.upstream.Algorithm,
		}
		totalTargets += len(state.targets)
		healthyTargets += len(healthy)
	}

	return map[string]interface{}{
		"status":          "healthy",
		"algorithm":       "round_robin",
		"upstream_count":  len(rb.upstreams),
		"total_targets":   totalTargets,
		"healthy_targets": healthyTargets,
		"upstreams":       upstreamHealth,
	}
}

// Metrics returns load balancer metrics
func (rb *RoundRobinBalancer) Metrics() map[string]interface{} {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	metrics := map[string]interface{}{
		"upstream_count": len(rb.upstreams),
	}

	// Add per-upstream metrics
	upstreamMetrics := make(map[string]interface{})
	for id, state := range rb.upstreams {
		upstreamMetrics[id] = map[string]interface{}{
			"target_count":    len(state.targets),
			"request_counter": atomic.LoadUint64(&state.counter),
		}
	}
	metrics["upstreams"] = upstreamMetrics

	return metrics
}

// getHealthyTargets returns only healthy targets
func (rb *RoundRobinBalancer) getHealthyTargets(targets []*types.Target) []*types.Target {
	healthy := make([]*types.Target, 0, len(targets))
	for _, target := range targets {
		if target.Healthy {
			healthy = append(healthy, target)
		}
	}
	return healthy
}

// validateUpstream validates upstream configuration
func (rb *RoundRobinBalancer) validateUpstream(upstream *types.Upstream) error {
	if upstream.ID == "" {
		return fmt.Errorf("upstream ID cannot be empty")
	}

	if upstream.Name == "" {
		return fmt.Errorf("upstream name cannot be empty")
	}

	if len(upstream.Targets) == 0 {
		return fmt.Errorf("upstream must have at least one target")
	}

	// Validate targets
	for i, target := range upstream.Targets {
		if target.Host == "" {
			return fmt.Errorf("target %d: host cannot be empty", i)
		}
		if target.Port <= 0 || target.Port > 65535 {
			return fmt.Errorf("target %d: invalid port %d", i, target.Port)
		}
		if target.Weight < 0 {
			return fmt.Errorf("target %d: weight cannot be negative", i)
		}
	}

	return nil
}

// Start starts the load balancer
func (rb *RoundRobinBalancer) Start() error {
	// Initialize with default configuration if needed
	return nil
}

// Stop stops the load balancer
func (rb *RoundRobinBalancer) Stop() error {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	// Clear all upstreams
	rb.upstreams = make(map[string]*upstreamState)
	return nil
}

// Reset resets all counters
func (rb *RoundRobinBalancer) Reset() {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	for _, state := range rb.upstreams {
		atomic.StoreUint64(&state.counter, 0)
	}
}
