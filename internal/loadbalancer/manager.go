package loadbalancer

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/health"
	"github.com/songzhibin97/stargate/internal/router"
	"github.com/songzhibin97/stargate/internal/types"
	"github.com/songzhibin97/stargate/pkg/discovery"
	discoveryManager "github.com/songzhibin97/stargate/internal/discovery"
)

// Manager manages multiple load balancers and provides hot reload capabilities
type Manager struct {
	config           *config.Config
	mu               sync.RWMutex
	balancers        map[string]types.LoadBalancer
	healthChecker    *health.ActiveHealthChecker
	defaultAlgo      string
	discoveryManager *discoveryManager.Manager
	serviceWatchers  map[string]context.CancelFunc // service name -> cancel function
	enableDiscovery  bool
	ctx              context.Context
	cancel           context.CancelFunc
	logger           *log.Logger
}

// NewManager creates a new load balancer manager
func NewManager(cfg *config.Config, healthChecker *health.ActiveHealthChecker, logger *log.Logger) *Manager {
	// Use standard log for now
	_ = logger

	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		config:          cfg,
		balancers:       make(map[string]types.LoadBalancer),
		healthChecker:   healthChecker,
		defaultAlgo:     "round_robin", // Default algorithm
		serviceWatchers: make(map[string]context.CancelFunc),
		enableDiscovery: false, // Disabled by default
		ctx:             ctx,
		cancel:          cancel,
		logger:          logger,
	}
}

// NewManagerWithDiscovery creates a new load balancer manager with service discovery
func NewManagerWithDiscovery(cfg *config.Config, healthChecker *health.ActiveHealthChecker, discoveryMgr *discoveryManager.Manager, logger *log.Logger) *Manager {
	// Use standard log for now
	_ = logger

	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		config:           cfg,
		balancers:        make(map[string]types.LoadBalancer),
		healthChecker:    healthChecker,
		defaultAlgo:      "round_robin",
		discoveryManager: discoveryMgr,
		serviceWatchers:  make(map[string]context.CancelFunc),
		enableDiscovery:  true,
		ctx:              ctx,
		cancel:           cancel,
		logger:           logger,
	}
}

// GetBalancer returns the appropriate load balancer for an upstream
func (m *Manager) GetBalancer(upstream *router.Upstream) (types.LoadBalancer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	algorithm := upstream.Algorithm
	if algorithm == "" {
		algorithm = m.defaultAlgo
	}

	balancer, exists := m.balancers[algorithm]
	if !exists {
		return nil, fmt.Errorf("load balancer algorithm %s not found", algorithm)
	}

	return balancer, nil
}

// RegisterBalancer registers a load balancer with an algorithm name
func (m *Manager) RegisterBalancer(algorithm string, balancer types.LoadBalancer) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.balancers[algorithm] = balancer
	log.Printf("Load balancer registered: algorithm=%s", algorithm)
}

// UpdateUpstream updates an upstream in the appropriate load balancer
func (m *Manager) UpdateUpstream(upstream *router.Upstream) error {
	// Convert router.Upstream to types.Upstream
	typesUpstream := m.convertUpstream(upstream)

	algorithm := upstream.Algorithm
	if algorithm == "" {
		algorithm = m.defaultAlgo
	}

	m.mu.RLock()
	balancer, exists := m.balancers[algorithm]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("load balancer algorithm %s not found", algorithm)
	}

	return balancer.UpdateUpstream(typesUpstream)
}

// DeleteUpstream removes an upstream from all load balancers
func (m *Manager) DeleteUpstream(upstreamID string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var lastError error
	for algorithm, balancer := range m.balancers {
		if err := balancer.RemoveUpstream(upstreamID); err != nil {
			log.Printf("Failed to remove upstream from balancer: upstream=%s, algorithm=%s, error=%v",
				 upstreamID,
				 algorithm,
				err,
			)
			lastError = err
		}
	}

	return lastError
}

// AddUpstream adds an upstream to the appropriate load balancer
func (m *Manager) AddUpstream(upstream *router.Upstream) error {
	return m.UpdateUpstream(upstream) // UpdateUpstream handles both add and update
}

// ClearUpstreams removes all upstreams from all load balancers
func (m *Manager) ClearUpstreams() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var lastError error
	for algorithm, balancer := range m.balancers {
		// Get all upstreams and remove them
		health := balancer.Health()
		if upstreamsCount, ok := health["upstreams_count"].(int); ok && upstreamsCount > 0 {
			// This is a simplified approach - in a real implementation,
			// we would need a method to list all upstream IDs
			log.Printf("Clearing upstreams from balancer: algorithm=%s, count=%d",
				 algorithm,
				 upstreamsCount,
			)
		}
	}

	return lastError
}

// EnableServiceDiscovery enables service discovery for the manager
func (m *Manager) EnableServiceDiscovery(discoveryMgr *discoveryManager.Manager) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.discoveryManager = discoveryMgr
	m.enableDiscovery = true

	log.Println("Service discovery enabled for load balancer manager")
}

// DisableServiceDiscovery disables service discovery for the manager
func (m *Manager) DisableServiceDiscovery() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop all service watchers
	for serviceName, cancel := range m.serviceWatchers {
		cancel()
		log.Printf("Stopped watching service: %s",
			 serviceName,
		)
	}
	m.serviceWatchers = make(map[string]context.CancelFunc)

	m.enableDiscovery = false
	m.discoveryManager = nil

	log.Println("Service discovery disabled for load balancer manager")
}

// WatchService starts watching a service for changes and updates load balancers accordingly
func (m *Manager) WatchService(serviceName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.enableDiscovery || m.discoveryManager == nil {
		return fmt.Errorf("service discovery is not enabled")
	}

	// Check if already watching
	if _, exists := m.serviceWatchers[serviceName]; exists {
		return fmt.Errorf("already watching service: %s", serviceName)
	}

	// Create context for this watcher
	ctx, cancel := context.WithCancel(m.ctx)
	m.serviceWatchers[serviceName] = cancel

	// Start watching
	callback := func(event *discovery.WatchEvent) {
		m.handleServiceEvent(serviceName, event)
	}

	if err := m.discoveryManager.WatchServiceFromDefault(ctx, serviceName, callback); err != nil {
		cancel()
		delete(m.serviceWatchers, serviceName)
		return fmt.Errorf("failed to watch service %s: %w", serviceName, err)
	}

	log.Printf("Started watching service: %s",
		 serviceName,
	)
	return nil
}

// UnwatchService stops watching a service
func (m *Manager) UnwatchService(serviceName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cancel, exists := m.serviceWatchers[serviceName]
	if !exists {
		return fmt.Errorf("not watching service: %s", serviceName)
	}

	cancel()
	delete(m.serviceWatchers, serviceName)

	log.Printf("Stopped watching service: %s",
		 serviceName,
	)
	return nil
}

// handleServiceEvent handles service discovery events
func (m *Manager) handleServiceEvent(serviceName string, event *discovery.WatchEvent) {
	log.Printf("Received service event: service=%s, type=%s, instances=%d",
		 serviceName,
		 string(event.Type),
		 len(event.Instances),
	)

	// Convert discovery instances to upstream targets
	upstream := m.convertServiceInstancesToUpstream(serviceName, event.Instances)

	// Update all load balancers with the new upstream
	m.mu.RLock()
	defer m.mu.RUnlock()

	for algorithm, balancer := range m.balancers {
		if err := balancer.UpdateUpstream(upstream); err != nil {
			log.Printf("Failed to update balancer with service: algorithm=%s, service=%s, error=%v",
				 algorithm,
				 serviceName,
				err,
			)
		} else {
			log.Printf("Updated balancer with service: algorithm=%s, service=%s, targets=%d",
				 algorithm,
				 serviceName,
				 len(upstream.Targets),
			)
		}
	}
}

// convertServiceInstancesToUpstream converts discovery service instances to types.Upstream
func (m *Manager) convertServiceInstancesToUpstream(serviceName string, instances []*discovery.ServiceInstance) *types.Upstream {
	targets := make([]*types.Target, 0, len(instances))

	for _, instance := range instances {
		target := &types.Target{
			Host:    instance.Host,
			Port:    instance.Port,
			Weight:  instance.Weight,
			Healthy: instance.Healthy,
		}
		targets = append(targets, target)
	}

	return &types.Upstream{
		ID:        serviceName,
		Name:      serviceName,
		Algorithm: m.defaultAlgo,
		Targets:   targets,
		Metadata:  map[string]string{
			"source": "service_discovery",
			"type":   "dynamic",
		},
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}
}

// SyncServiceFromDiscovery synchronizes a service from service discovery
func (m *Manager) SyncServiceFromDiscovery(serviceName string) error {
	if !m.enableDiscovery || m.discoveryManager == nil {
		return fmt.Errorf("service discovery is not enabled")
	}

	// Get current service instances
	instances, err := m.discoveryManager.GetServiceInstancesFromDefault(m.ctx, serviceName)
	if err != nil {
		return fmt.Errorf("failed to get service instances for %s: %w", serviceName, err)
	}

	// Convert to upstream and update balancers
	upstream := m.convertServiceInstancesToUpstream(serviceName, instances)

	m.mu.RLock()
	defer m.mu.RUnlock()

	for algorithm, balancer := range m.balancers {
		if err := balancer.UpdateUpstream(upstream); err != nil {
			log.Printf("Failed to sync %s balancer with service %s: %v",
				algorithm, serviceName, err)
		} else {
			log.Printf("Synced %s balancer with service %s (%d targets)",
				algorithm, serviceName, len(upstream.Targets))
		}
	}

	return nil
}

// ReloadUpstreams reloads all upstreams
func (m *Manager) ReloadUpstreams(upstreams []router.Upstream) error {
	// Clear existing upstreams
	if err := m.ClearUpstreams(); err != nil {
		log.Printf("Failed to clear existing upstreams: %v", err)
	}

	// Add all new upstreams
	var lastError error
	for _, upstream := range upstreams {
		if err := m.UpdateUpstream(&upstream); err != nil {
			log.Printf("Failed to reload upstream %s: %v", upstream.ID, err)
			lastError = err
		}
	}

	log.Printf("Reloaded upstreams: count=%d",
		 len(upstreams),
	)
	return lastError
}

// convertUpstream converts router.Upstream to types.Upstream
func (m *Manager) convertUpstream(upstream *router.Upstream) *types.Upstream {
	targets := make([]*types.Target, len(upstream.Targets))
	for i, target := range upstream.Targets {
		targets[i] = &types.Target{
			Host:    target.URL, // Assuming URL contains host:port
			Port:    target.Weight, // This needs proper parsing
			Weight:  target.Weight,
			Healthy: true, // Default to healthy
		}
	}

	return &types.Upstream{
		ID:        upstream.ID,
		Name:      upstream.Name,
		Algorithm: upstream.Algorithm,
		Targets:   targets,
		Metadata:  upstream.Metadata,
		CreatedAt: upstream.CreatedAt,
		UpdatedAt: upstream.UpdatedAt,
	}
}

// InitializeBalancers initializes all supported load balancers
func (m *Manager) InitializeBalancers() error {
	// Initialize Round Robin balancer
	rrBalancer := NewRoundRobinBalancer(m.config)
	m.RegisterBalancer("round_robin", rrBalancer)

	// Initialize Weighted Round Robin balancer
	wrrBalancer := NewWeightedRoundRobinBalancer(m.config)
	m.RegisterBalancer("weighted_round_robin", wrrBalancer)

	// Initialize IP Hash balancer
	ipHashBalancer := NewIPHashBalancer(m.config)
	m.RegisterBalancer("ip_hash", ipHashBalancer)

	// Initialize Canary balancer
	canaryBalancer := NewCanaryBalancer(m.config)
	m.RegisterBalancer("canary", canaryBalancer)

	log.Printf("All load balancers initialized")
	return nil
}

// Select selects a target from the appropriate load balancer
func (m *Manager) Select(upstream *router.Upstream) (*types.Target, error) {
	balancer, err := m.GetBalancer(upstream)
	if err != nil {
		return nil, err
	}

	typesUpstream := m.convertUpstream(upstream)
	return balancer.Select(typesUpstream)
}

// Health returns the health status of all load balancers and service discovery
func (m *Manager) Health() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	health := map[string]interface{}{
		"balancers_count": len(m.balancers),
		"default_algo":    m.defaultAlgo,
		"timestamp":       time.Now().Unix(),
	}

	// Load balancer health
	balancerHealth := make(map[string]interface{})
	for algorithm, balancer := range m.balancers {
		balancerHealth[algorithm] = balancer.Health()
	}
	health["balancers"] = balancerHealth

	// Service discovery health
	discoveryHealth := map[string]interface{}{
		"enabled":         m.enableDiscovery,
		"watched_services": len(m.serviceWatchers),
	}

	if m.enableDiscovery && m.discoveryManager != nil {
		// Get service discovery health
		registryHealth := m.discoveryManager.HealthCheck(m.ctx)
		discoveryHealth["registries"] = registryHealth

		// List watched services
		watchedServices := make([]string, 0, len(m.serviceWatchers))
		for serviceName := range m.serviceWatchers {
			watchedServices = append(watchedServices, serviceName)
		}
		discoveryHealth["services"] = watchedServices
	}

	health["service_discovery"] = discoveryHealth
	return health
}

// GetSupportedAlgorithms returns a list of supported load balancing algorithms
func (m *Manager) GetSupportedAlgorithms() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	algorithms := make([]string, 0, len(m.balancers))
	for algorithm := range m.balancers {
		algorithms = append(algorithms, algorithm)
	}

	return algorithms
}

// SetDefaultAlgorithm sets the default load balancing algorithm
func (m *Manager) SetDefaultAlgorithm(algorithm string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.balancers[algorithm]; !exists {
		return fmt.Errorf("algorithm %s is not registered", algorithm)
	}

	m.defaultAlgo = algorithm
	log.Printf("Default load balancing algorithm updated: %s",
		 algorithm,
	)
	return nil
}

// Start starts all load balancers
func (m *Manager) Start() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for algorithm, balancer := range m.balancers {
		if starter, ok := balancer.(interface{ Start() error }); ok {
			if err := starter.Start(); err != nil {
				return fmt.Errorf("failed to start %s balancer: %w", algorithm, err)
			}
		}
	}

	log.Println("All load balancers started")
	return nil
}

// Stop stops all load balancers and service discovery watchers
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop service discovery watchers first
	for serviceName, cancel := range m.serviceWatchers {
		cancel()
		log.Printf("Stopped watching service: %s", serviceName)
	}
	m.serviceWatchers = make(map[string]context.CancelFunc)

	// Cancel main context
	if m.cancel != nil {
		m.cancel()
	}

	// Stop all load balancers
	var lastError error
	for algorithm, balancer := range m.balancers {
		if stopper, ok := balancer.(interface{ Stop() error }); ok {
			if err := stopper.Stop(); err != nil {
				log.Printf("Failed to stop %s balancer: %v", algorithm, err)
				lastError = err
			}
		}
	}

	log.Println("All load balancers and service watchers stopped")
	return lastError
}

// GetWatchedServices returns a list of services being watched
func (m *Manager) GetWatchedServices() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	services := make([]string, 0, len(m.serviceWatchers))
	for serviceName := range m.serviceWatchers {
		services = append(services, serviceName)
	}

	return services
}

// IsServiceDiscoveryEnabled returns whether service discovery is enabled
func (m *Manager) IsServiceDiscoveryEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.enableDiscovery && m.discoveryManager != nil
}

// GetServiceInstances gets current service instances from service discovery
func (m *Manager) GetServiceInstances(serviceName string) ([]*discovery.ServiceInstance, error) {
	if !m.enableDiscovery || m.discoveryManager == nil {
		return nil, fmt.Errorf("service discovery is not enabled")
	}

	return m.discoveryManager.GetServiceInstancesFromDefault(m.ctx, serviceName)
}

// RefreshService manually refreshes a service from service discovery
func (m *Manager) RefreshService(serviceName string) error {
	if !m.enableDiscovery || m.discoveryManager == nil {
		return fmt.Errorf("service discovery is not enabled")
	}

	log.Printf("Manually refreshing service: %s", serviceName)
	return m.SyncServiceFromDiscovery(serviceName)
}

// AddUpstreamFromDiscovery adds an upstream from service discovery and starts watching it
func (m *Manager) AddUpstreamFromDiscovery(serviceName string) error {
	if !m.enableDiscovery || m.discoveryManager == nil {
		return fmt.Errorf("service discovery is not enabled")
	}

	// First sync the service to get initial instances
	if err := m.SyncServiceFromDiscovery(serviceName); err != nil {
		return fmt.Errorf("failed to sync service %s: %w", serviceName, err)
	}

	// Then start watching for changes
	if err := m.WatchService(serviceName); err != nil {
		return fmt.Errorf("failed to watch service %s: %w", serviceName, err)
	}

	log.Printf("Added upstream from service discovery: %s", serviceName)
	return nil
}

// RemoveUpstreamFromDiscovery removes an upstream and stops watching it
func (m *Manager) RemoveUpstreamFromDiscovery(serviceName string) error {
	// Stop watching the service
	if err := m.UnwatchService(serviceName); err != nil {
		log.Printf("Warning: failed to stop watching service %s: %v", serviceName, err)
	}

	// Remove from all load balancers
	if err := m.DeleteUpstream(serviceName); err != nil {
		return fmt.Errorf("failed to remove upstream %s: %w", serviceName, err)
	}

	log.Printf("Removed upstream from service discovery: %s", serviceName)
	return nil
}
