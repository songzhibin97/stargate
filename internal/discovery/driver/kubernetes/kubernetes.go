package kubernetes

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/songzhibin97/stargate/pkg/discovery"
)

// KubernetesRegistry implements the discovery.Registry interface using Kubernetes API
type KubernetesRegistry struct {
	config       *discovery.Config
	clientset    kubernetes.Interface
	services     map[string][]*discovery.ServiceInstance
	watchers     map[string][]discovery.WatchCallback
	mu           sync.RWMutex
	stopCh       chan struct{}
	wg           sync.WaitGroup
	started      bool
	namespace    string
	useEndpoints bool // true for Endpoints, false for EndpointSlices
}

// KubernetesConfig represents the Kubernetes-specific configuration
type KubernetesConfig struct {
	// Kubeconfig path (optional, uses in-cluster config if empty)
	Kubeconfig string `yaml:"kubeconfig" json:"kubeconfig"`
	
	// Namespace to watch (empty means all namespaces)
	Namespace string `yaml:"namespace" json:"namespace"`
	
	// UseEndpoints determines whether to use Endpoints (true) or EndpointSlices (false)
	UseEndpoints bool `yaml:"use_endpoints" json:"use_endpoints"`
	
	// LabelSelector for filtering services
	LabelSelector string `yaml:"label_selector" json:"label_selector"`
	
	// FieldSelector for filtering services
	FieldSelector string `yaml:"field_selector" json:"field_selector"`
	
	// ResyncPeriod for periodic full resync
	ResyncPeriod time.Duration `yaml:"resync_period" json:"resync_period"`
}

// New creates a new Kubernetes service discovery registry
func New(config *discovery.Config) (discovery.Registry, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Parse Kubernetes-specific configuration
	k8sConfig, err := parseKubernetesConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse kubernetes config: %w", err)
	}

	// Create Kubernetes client
	clientset, err := createKubernetesClient(k8sConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	registry := &KubernetesRegistry{
		config:       config,
		clientset:    clientset,
		services:     make(map[string][]*discovery.ServiceInstance),
		watchers:     make(map[string][]discovery.WatchCallback),
		stopCh:       make(chan struct{}),
		namespace:    k8sConfig.Namespace,
		useEndpoints: k8sConfig.UseEndpoints,
	}

	// Start watching for changes
	if err := registry.start(); err != nil {
		return nil, fmt.Errorf("failed to start kubernetes registry: %w", err)
	}

	return registry, nil
}

// GetService retrieves service instances by service name
func (r *KubernetesRegistry) GetService(ctx context.Context, serviceName string) ([]*discovery.ServiceInstance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	instances, exists := r.services[serviceName]
	if !exists {
		return []*discovery.ServiceInstance{}, nil
	}

	// Return a copy to prevent external modification
	result := make([]*discovery.ServiceInstance, len(instances))
	for i, instance := range instances {
		result[i] = r.copyInstance(instance)
	}

	return result, nil
}

// Watch watches for service changes and calls the callback when changes occur
func (r *KubernetesRegistry) Watch(ctx context.Context, serviceName string, callback discovery.WatchCallback) error {
	if callback == nil {
		return fmt.Errorf("callback cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Add callback to watchers
	r.watchers[serviceName] = append(r.watchers[serviceName], callback)

	// Send initial event with current instances
	instances, exists := r.services[serviceName]
	if exists && len(instances) > 0 {
		go func() {
			event := &discovery.WatchEvent{
				Type:        discovery.EventTypeServiceUpdated,
				ServiceName: serviceName,
				Instances:   r.copyInstances(instances),
				Timestamp:   time.Now(),
			}
			callback(event)
		}()
	}

	return nil
}

// Unwatch stops watching a service
func (r *KubernetesRegistry) Unwatch(serviceName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.watchers, serviceName)
	return nil
}

// Close closes the service discovery client and releases resources
func (r *KubernetesRegistry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.started {
		return nil
	}

	close(r.stopCh)
	r.started = false

	// Wait for watchers to finish
	r.wg.Wait()

	// Clear all data
	r.services = make(map[string][]*discovery.ServiceInstance)
	r.watchers = make(map[string][]discovery.WatchCallback)

	return nil
}

// Health returns the health status of the service discovery client
func (r *KubernetesRegistry) Health(ctx context.Context) *discovery.HealthStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	status := "healthy"
	message := "Kubernetes registry is operational"

	// Test connection to Kubernetes API
	if _, err := r.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1}); err != nil {
		status = "unhealthy"
		message = fmt.Sprintf("Failed to connect to Kubernetes API: %v", err)
	}

	return &discovery.HealthStatus{
		Status:    status,
		Message:   message,
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"type":           "kubernetes",
			"namespace":      r.namespace,
			"use_endpoints":  r.useEndpoints,
			"services_count": len(r.services),
			"started":        r.started,
		},
	}
}

// ListServices lists all available services
func (r *KubernetesRegistry) ListServices(ctx context.Context) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	services := make([]string, 0, len(r.services))
	for serviceName := range r.services {
		services = append(services, serviceName)
	}

	return services, nil
}

// start starts the Kubernetes watchers
func (r *KubernetesRegistry) start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.started {
		return nil
	}

	r.started = true

	// Start watching based on configuration
	if r.useEndpoints {
		r.wg.Add(1)
		go r.watchEndpoints()
	} else {
		r.wg.Add(1)
		go r.watchEndpointSlices()
	}

	// Also watch Services for metadata
	r.wg.Add(1)
	go r.watchServices()

	return nil
}

// parseKubernetesConfig parses Kubernetes-specific configuration from discovery config
func parseKubernetesConfig(config *discovery.Config) (*KubernetesConfig, error) {
	k8sConfig := &KubernetesConfig{
		UseEndpoints: true, // Default to Endpoints for backward compatibility
		ResyncPeriod: 30 * time.Second,
	}

	// Parse from options
	if kubeconfig, ok := config.Options["kubeconfig"].(string); ok {
		k8sConfig.Kubeconfig = kubeconfig
	}

	if namespace, ok := config.Options["namespace"].(string); ok {
		k8sConfig.Namespace = namespace
	}

	if useEndpoints, ok := config.Options["use_endpoints"].(bool); ok {
		k8sConfig.UseEndpoints = useEndpoints
	}

	if labelSelector, ok := config.Options["label_selector"].(string); ok {
		k8sConfig.LabelSelector = labelSelector
	}

	if fieldSelector, ok := config.Options["field_selector"].(string); ok {
		k8sConfig.FieldSelector = fieldSelector
	}

	if resyncPeriod, ok := config.Options["resync_period"].(time.Duration); ok {
		k8sConfig.ResyncPeriod = resyncPeriod
	}

	return k8sConfig, nil
}

// createKubernetesClient creates a Kubernetes client based on configuration
func createKubernetesClient(config *KubernetesConfig) (kubernetes.Interface, error) {
	var restConfig *rest.Config
	var err error

	if config.Kubeconfig != "" {
		// Use kubeconfig file
		restConfig, err = clientcmd.BuildConfigFromFlags("", config.Kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
		}
	} else {
		// Use in-cluster config
		restConfig, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
		}
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	return clientset, nil
}

// watchEndpoints watches Kubernetes Endpoints for changes
func (r *KubernetesRegistry) watchEndpoints() {
	defer r.wg.Done()

	for {
		select {
		case <-r.stopCh:
			return
		default:
			if err := r.doWatchEndpoints(); err != nil {
				// Log error and retry after a delay
				time.Sleep(5 * time.Second)
			}
		}
	}
}

// doWatchEndpoints performs the actual Endpoints watching
func (r *KubernetesRegistry) doWatchEndpoints() error {
	listOptions := metav1.ListOptions{
		Watch: true,
	}

	watcher, err := r.clientset.CoreV1().Endpoints(r.namespace).Watch(context.Background(), listOptions)
	if err != nil {
		return fmt.Errorf("failed to watch endpoints: %w", err)
	}
	defer watcher.Stop()

	for {
		select {
		case <-r.stopCh:
			return nil
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("endpoints watch channel closed")
			}

			if err := r.handleEndpointsEvent(event); err != nil {
				// Log error but continue watching
				continue
			}
		}
	}
}

// handleEndpointsEvent handles a single Endpoints event
func (r *KubernetesRegistry) handleEndpointsEvent(event watch.Event) error {
	endpoints, ok := event.Object.(*corev1.Endpoints)
	if !ok {
		return fmt.Errorf("unexpected object type: %T", event.Object)
	}

	serviceName := endpoints.Name
	namespace := endpoints.Namespace

	switch event.Type {
	case watch.Added, watch.Modified:
		instances := r.convertEndpointsToInstances(endpoints)
		r.updateServiceInstances(serviceName, namespace, instances)
	case watch.Deleted:
		r.removeServiceInstances(serviceName, namespace)
	}

	return nil
}

// watchEndpointSlices watches Kubernetes EndpointSlices for changes
func (r *KubernetesRegistry) watchEndpointSlices() {
	defer r.wg.Done()

	for {
		select {
		case <-r.stopCh:
			return
		default:
			if err := r.doWatchEndpointSlices(); err != nil {
				// Log error and retry after a delay
				time.Sleep(5 * time.Second)
			}
		}
	}
}

// doWatchEndpointSlices performs the actual EndpointSlices watching
func (r *KubernetesRegistry) doWatchEndpointSlices() error {
	listOptions := metav1.ListOptions{
		Watch: true,
	}

	watcher, err := r.clientset.DiscoveryV1().EndpointSlices(r.namespace).Watch(context.Background(), listOptions)
	if err != nil {
		return fmt.Errorf("failed to watch endpointslices: %w", err)
	}
	defer watcher.Stop()

	for {
		select {
		case <-r.stopCh:
			return nil
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("endpointslices watch channel closed")
			}

			if err := r.handleEndpointSlicesEvent(event); err != nil {
				// Log error but continue watching
				continue
			}
		}
	}
}

// handleEndpointSlicesEvent handles a single EndpointSlices event
func (r *KubernetesRegistry) handleEndpointSlicesEvent(event watch.Event) error {
	endpointSlice, ok := event.Object.(*discoveryv1.EndpointSlice)
	if !ok {
		return fmt.Errorf("unexpected object type: %T", event.Object)
	}

	serviceName := endpointSlice.Labels[discoveryv1.LabelServiceName]
	if serviceName == "" {
		return fmt.Errorf("endpointslice missing service name label")
	}

	namespace := endpointSlice.Namespace

	switch event.Type {
	case watch.Added, watch.Modified:
		instances := r.convertEndpointSlicesToInstances(endpointSlice)
		r.updateServiceInstances(serviceName, namespace, instances)
	case watch.Deleted:
		r.removeServiceInstances(serviceName, namespace)
	}

	return nil
}

// watchServices watches Kubernetes Services for metadata
func (r *KubernetesRegistry) watchServices() {
	defer r.wg.Done()

	for {
		select {
		case <-r.stopCh:
			return
		default:
			if err := r.doWatchServices(); err != nil {
				// Log error and retry after a delay
				time.Sleep(5 * time.Second)
			}
		}
	}
}

// doWatchServices performs the actual Services watching
func (r *KubernetesRegistry) doWatchServices() error {
	listOptions := metav1.ListOptions{
		Watch: true,
	}

	watcher, err := r.clientset.CoreV1().Services(r.namespace).Watch(context.Background(), listOptions)
	if err != nil {
		return fmt.Errorf("failed to watch services: %w", err)
	}
	defer watcher.Stop()

	for {
		select {
		case <-r.stopCh:
			return nil
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("services watch channel closed")
			}

			if err := r.handleServiceEvent(event); err != nil {
				// Log error but continue watching
				continue
			}
		}
	}
}

// handleServiceEvent handles a single Service event
func (r *KubernetesRegistry) handleServiceEvent(event watch.Event) error {
	service, ok := event.Object.(*corev1.Service)
	if !ok {
		return fmt.Errorf("unexpected object type: %T", event.Object)
	}

	serviceName := service.Name
	namespace := service.Namespace

	switch event.Type {
	case watch.Added, watch.Modified:
		// Update service metadata for existing instances
		r.updateServiceMetadata(serviceName, namespace, service)
	case watch.Deleted:
		// Service deleted, remove all instances
		r.removeServiceInstances(serviceName, namespace)
	}

	return nil
}

// convertEndpointsToInstances converts Kubernetes Endpoints to service instances
func (r *KubernetesRegistry) convertEndpointsToInstances(endpoints *corev1.Endpoints) []*discovery.ServiceInstance {
	var instances []*discovery.ServiceInstance

	for _, subset := range endpoints.Subsets {
		for _, address := range subset.Addresses {
			for _, port := range subset.Ports {
				instance := &discovery.ServiceInstance{
					ID:            fmt.Sprintf("%s:%d", address.IP, port.Port),
					ServiceName:   endpoints.Name,
					Host:          address.IP,
					Port:          int(port.Port),
					Weight:        1, // Default weight
					Priority:      0, // Default priority
					Healthy:       true,
					Tags:          r.extractTags(endpoints.Labels, endpoints.Annotations),
					Metadata:      r.extractMetadata(endpoints.Labels, endpoints.Annotations),
					Status:        discovery.InstanceStatusUp,
					RegisterTime:  endpoints.CreationTimestamp.Time,
					LastHeartbeat: time.Now(),
					Zone:          r.extractZone(address),
					Region:        r.extractRegion(address),
				}

				// Add target reference information if available
				if address.TargetRef != nil {
					if instance.Metadata == nil {
						instance.Metadata = make(map[string]string)
					}
					instance.Metadata["target_kind"] = address.TargetRef.Kind
					instance.Metadata["target_name"] = address.TargetRef.Name
					instance.Metadata["target_namespace"] = address.TargetRef.Namespace
				}

				instances = append(instances, instance)
			}
		}

		// Handle not ready addresses
		for _, address := range subset.NotReadyAddresses {
			for _, port := range subset.Ports {
				instance := &discovery.ServiceInstance{
					ID:            fmt.Sprintf("%s:%d", address.IP, port.Port),
					ServiceName:   endpoints.Name,
					Host:          address.IP,
					Port:          int(port.Port),
					Weight:        1,
					Priority:      0,
					Healthy:       false,
					Tags:          r.extractTags(endpoints.Labels, endpoints.Annotations),
					Metadata:      r.extractMetadata(endpoints.Labels, endpoints.Annotations),
					Status:        discovery.InstanceStatusDown,
					RegisterTime:  endpoints.CreationTimestamp.Time,
					LastHeartbeat: time.Now(),
					Zone:          r.extractZone(address),
					Region:        r.extractRegion(address),
				}

				if address.TargetRef != nil {
					if instance.Metadata == nil {
						instance.Metadata = make(map[string]string)
					}
					instance.Metadata["target_kind"] = address.TargetRef.Kind
					instance.Metadata["target_name"] = address.TargetRef.Name
					instance.Metadata["target_namespace"] = address.TargetRef.Namespace
				}

				instances = append(instances, instance)
			}
		}
	}

	return instances
}

// convertEndpointSlicesToInstances converts Kubernetes EndpointSlices to service instances
func (r *KubernetesRegistry) convertEndpointSlicesToInstances(endpointSlice *discoveryv1.EndpointSlice) []*discovery.ServiceInstance {
	var instances []*discovery.ServiceInstance

	serviceName := endpointSlice.Labels[discoveryv1.LabelServiceName]

	for _, endpoint := range endpointSlice.Endpoints {
		for _, address := range endpoint.Addresses {
			for _, port := range endpointSlice.Ports {
				if port.Port == nil {
					continue
				}

				healthy := endpoint.Conditions.Ready != nil && *endpoint.Conditions.Ready
				status := discovery.InstanceStatusUp
				if !healthy {
					status = discovery.InstanceStatusDown
				}

				instance := &discovery.ServiceInstance{
					ID:            fmt.Sprintf("%s:%d", address, *port.Port),
					ServiceName:   serviceName,
					Host:          address,
					Port:          int(*port.Port),
					Weight:        1,
					Priority:      0,
					Healthy:       healthy,
					Tags:          r.extractTags(endpointSlice.Labels, endpointSlice.Annotations),
					Metadata:      r.extractMetadata(endpointSlice.Labels, endpointSlice.Annotations),
					Status:        status,
					RegisterTime:  endpointSlice.CreationTimestamp.Time,
					LastHeartbeat: time.Now(),
					Zone:          r.extractZoneFromEndpoint(endpoint),
					Region:        r.extractRegionFromEndpoint(endpoint),
				}

				// Add endpoint-specific metadata
				if endpoint.TargetRef != nil {
					if instance.Metadata == nil {
						instance.Metadata = make(map[string]string)
					}
					instance.Metadata["target_kind"] = endpoint.TargetRef.Kind
					instance.Metadata["target_name"] = endpoint.TargetRef.Name
					instance.Metadata["target_namespace"] = endpoint.TargetRef.Namespace
				}

				if endpoint.NodeName != nil {
					if instance.Metadata == nil {
						instance.Metadata = make(map[string]string)
					}
					instance.Metadata["node_name"] = *endpoint.NodeName
				}

				instances = append(instances, instance)
			}
		}
	}

	return instances
}

// updateServiceInstances updates the service instances for a given service
func (r *KubernetesRegistry) updateServiceInstances(serviceName, namespace string, instances []*discovery.ServiceInstance) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Create full service name with namespace
	fullServiceName := r.getFullServiceName(serviceName, namespace)

	// Update instances
	oldInstances := r.services[fullServiceName]
	r.services[fullServiceName] = instances

	// Notify watchers
	r.notifyWatchers(fullServiceName, oldInstances, instances)
}

// removeServiceInstances removes all instances for a given service
func (r *KubernetesRegistry) removeServiceInstances(serviceName, namespace string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	fullServiceName := r.getFullServiceName(serviceName, namespace)
	oldInstances := r.services[fullServiceName]
	delete(r.services, fullServiceName)

	// Notify watchers about removal
	r.notifyWatchers(fullServiceName, oldInstances, nil)
}

// updateServiceMetadata updates metadata for existing service instances
func (r *KubernetesRegistry) updateServiceMetadata(serviceName, namespace string, service *corev1.Service) {
	r.mu.Lock()
	defer r.mu.Unlock()

	fullServiceName := r.getFullServiceName(serviceName, namespace)
	instances, exists := r.services[fullServiceName]
	if !exists {
		return
	}

	// Update metadata for all instances
	for _, instance := range instances {
		if instance.Metadata == nil {
			instance.Metadata = make(map[string]string)
		}

		// Add service-level metadata
		for key, value := range service.Labels {
			instance.Metadata["service.label."+key] = value
		}

		for key, value := range service.Annotations {
			instance.Metadata["service.annotation."+key] = value
		}

		instance.Metadata["service_type"] = string(service.Spec.Type)
		if service.Spec.ClusterIP != "" {
			instance.Metadata["cluster_ip"] = service.Spec.ClusterIP
		}
	}
}

// getFullServiceName creates a full service name including namespace
func (r *KubernetesRegistry) getFullServiceName(serviceName, namespace string) string {
	if namespace == "" || namespace == "default" {
		return serviceName
	}
	return fmt.Sprintf("%s.%s", serviceName, namespace)
}

// notifyWatchers notifies all watchers about service changes
func (r *KubernetesRegistry) notifyWatchers(serviceName string, oldInstances, newInstances []*discovery.ServiceInstance) {
	callbacks, exists := r.watchers[serviceName]
	if !exists || len(callbacks) == 0 {
		return
	}

	var eventType discovery.EventType
	if oldInstances == nil && newInstances != nil {
		eventType = discovery.EventTypeServiceAdded
	} else if oldInstances != nil && newInstances == nil {
		eventType = discovery.EventTypeServiceRemoved
	} else {
		eventType = discovery.EventTypeServiceUpdated
	}

	event := &discovery.WatchEvent{
		Type:        eventType,
		ServiceName: serviceName,
		Instances:   r.copyInstances(newInstances),
		Timestamp:   time.Now(),
	}

	// Notify all callbacks for this service
	for _, callback := range callbacks {
		go func(cb discovery.WatchCallback, evt *discovery.WatchEvent) {
			cb(evt)
		}(callback, event)
	}
}

// extractTags extracts tags from Kubernetes labels and annotations
func (r *KubernetesRegistry) extractTags(labels, annotations map[string]string) map[string]string {
	tags := make(map[string]string)

	// Add labels as tags with "label." prefix
	for key, value := range labels {
		tags["label."+key] = value
	}

	// Add specific annotations as tags
	for key, value := range annotations {
		if strings.HasPrefix(key, "stargate.io/") || strings.HasPrefix(key, "discovery.") {
			tags["annotation."+key] = value
		}
	}

	return tags
}

// extractMetadata extracts metadata from Kubernetes labels and annotations
func (r *KubernetesRegistry) extractMetadata(labels, annotations map[string]string) map[string]string {
	metadata := make(map[string]string)

	// Add all labels as metadata
	for key, value := range labels {
		metadata["label."+key] = value
	}

	// Add all annotations as metadata
	for key, value := range annotations {
		metadata["annotation."+key] = value
	}

	return metadata
}

// extractZone extracts zone information from endpoint address
func (r *KubernetesRegistry) extractZone(address corev1.EndpointAddress) string {
	if address.NodeName != nil {
		// Try to get zone from node labels (would require additional API call)
		// For now, return empty string
		return ""
	}
	return ""
}

// extractRegion extracts region information from endpoint address
func (r *KubernetesRegistry) extractRegion(address corev1.EndpointAddress) string {
	if address.NodeName != nil {
		// Try to get region from node labels (would require additional API call)
		// For now, return empty string
		return ""
	}
	return ""
}

// extractZoneFromEndpoint extracts zone information from EndpointSlice endpoint
func (r *KubernetesRegistry) extractZoneFromEndpoint(endpoint discoveryv1.Endpoint) string {
	if endpoint.Zone != nil {
		return *endpoint.Zone
	}
	return ""
}

// extractRegionFromEndpoint extracts region information from EndpointSlice endpoint
func (r *KubernetesRegistry) extractRegionFromEndpoint(endpoint discoveryv1.Endpoint) string {
	// Region information is typically stored in node labels
	// For now, return empty string as it would require additional API calls
	return ""
}

// copyInstance creates a deep copy of a service instance
func (r *KubernetesRegistry) copyInstance(instance *discovery.ServiceInstance) *discovery.ServiceInstance {
	if instance == nil {
		return nil
	}

	return &discovery.ServiceInstance{
		ID:            instance.ID,
		ServiceName:   instance.ServiceName,
		Host:          instance.Host,
		Port:          instance.Port,
		Weight:        instance.Weight,
		Priority:      instance.Priority,
		Healthy:       instance.Healthy,
		Tags:          r.copyStringMap(instance.Tags),
		Metadata:      r.copyStringMap(instance.Metadata),
		Status:        instance.Status,
		RegisterTime:  instance.RegisterTime,
		LastHeartbeat: instance.LastHeartbeat,
		Version:       instance.Version,
		Zone:          instance.Zone,
		Region:        instance.Region,
	}
}

// copyInstances creates a deep copy of service instances slice
func (r *KubernetesRegistry) copyInstances(instances []*discovery.ServiceInstance) []*discovery.ServiceInstance {
	if instances == nil {
		return nil
	}

	result := make([]*discovery.ServiceInstance, len(instances))
	for i, instance := range instances {
		result[i] = r.copyInstance(instance)
	}
	return result
}

// copyStringMap creates a deep copy of a string map
func (r *KubernetesRegistry) copyStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}

	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}
