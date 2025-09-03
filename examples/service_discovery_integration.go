package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/health"
	"github.com/songzhibin97/stargate/internal/loadbalancer"
	"github.com/songzhibin97/stargate/pkg/discovery"
	discoveryManager "github.com/songzhibin97/stargate/internal/discovery"
)

func main() {
	log.Println("Starting Service Discovery Integration Example")

	// Create discovery manager configuration
	discoveryConfig := &discoveryManager.ManagerConfig{
		DefaultRegistry: "static",
		Registries: map[string]*discovery.Config{
			"static": {
				Type:            "static",
				Timeout:         5 * time.Second,
				RefreshInterval: 30 * time.Second,
				RetryCount:      3,
				RetryInterval:   1 * time.Second,
				Options: map[string]interface{}{
					"config_path": "examples/services.yaml",
				},
			},
			"kubernetes": {
				Type:            "kubernetes",
				Timeout:         30 * time.Second,
				RefreshInterval: 0, // Real-time watching
				RetryCount:      3,
				RetryInterval:   5 * time.Second,
				Options: map[string]interface{}{
					"namespace":     "default",
					"use_endpoints": true,
				},
			},
		},
		HealthCheckInterval: 30 * time.Second,
		EnableMetrics:       true,
	}

	// Create discovery manager
	discoveryMgr := discoveryManager.NewManager(discoveryConfig)

	// Start discovery manager
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := discoveryMgr.Start(ctx); err != nil {
		log.Fatalf("Failed to start discovery manager: %v", err)
	}
	defer discoveryMgr.Stop()

	// Create load balancer configuration
	lbConfig := &config.Config{
		// Add your load balancer configuration here
	}

	// Create health checker
	healthChecker := &health.ActiveHealthChecker{}

	// Create load balancer manager with service discovery
	logger := log.New(os.Stdout, "[LoadBalancer] ", log.LstdFlags)
	lbManager := loadbalancer.NewManagerWithDiscovery(lbConfig, healthChecker, discoveryMgr, logger)

	// Initialize load balancers
	if err := lbManager.InitializeBalancers(); err != nil {
		log.Fatalf("Failed to initialize load balancers: %v", err)
	}

	// Start load balancer manager
	if err := lbManager.Start(); err != nil {
		log.Fatalf("Failed to start load balancer manager: %v", err)
	}
	defer lbManager.Stop()

	// Example: Add services from service discovery
	services := []string{"web-service", "api-service", "database-service"}
	
	for _, serviceName := range services {
		log.Printf("Adding service from discovery: %s", serviceName)
		if err := lbManager.AddUpstreamFromDiscovery(serviceName); err != nil {
			log.Printf("Warning: Failed to add service %s: %v", serviceName, err)
		} else {
			log.Printf("Successfully added and watching service: %s", serviceName)
		}
	}

	// Print initial health status
	printHealthStatus(lbManager)

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start monitoring routine
	go monitorServices(lbManager)

	// Wait for shutdown signal
	<-sigChan
	log.Println("Received shutdown signal, stopping services...")

	// Graceful shutdown
	cancel()
	log.Println("Service Discovery Integration Example stopped")
}

// printHealthStatus prints the current health status
func printHealthStatus(manager *loadbalancer.Manager) {
	log.Println("=== Health Status ===")
	health := manager.Health()
	
	log.Printf("Load Balancers: %d", health["balancers_count"])
	log.Printf("Default Algorithm: %s", health["default_algo"])
	
	if discoveryHealth, ok := health["service_discovery"].(map[string]interface{}); ok {
		log.Printf("Service Discovery Enabled: %v", discoveryHealth["enabled"])
		log.Printf("Watched Services: %d", discoveryHealth["watched_services"])
		
		if services, ok := discoveryHealth["services"].([]string); ok {
			log.Printf("Services: %v", services)
		}
	}
	
	log.Println("=====================")
}

// monitorServices monitors service changes and prints updates
func monitorServices(manager *loadbalancer.Manager) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Println("--- Service Monitor Update ---")
			
			// Print watched services
			watchedServices := manager.GetWatchedServices()
			log.Printf("Currently watching %d services: %v", len(watchedServices), watchedServices)
			
			// Print health status
			printHealthStatus(manager)
			
			// Example: Manually refresh a service
			if len(watchedServices) > 0 {
				serviceName := watchedServices[0]
				log.Printf("Manually refreshing service: %s", serviceName)
				if err := manager.RefreshService(serviceName); err != nil {
					log.Printf("Failed to refresh service %s: %v", serviceName, err)
				}
			}
		}
	}
}

// Example configuration for services.yaml
const exampleServicesYAML = `
services:
  web-service:
    instances:
      - id: web-1
        host: 192.168.1.10
        port: 8080
        weight: 1
        healthy: true
        tags:
          env: production
          version: "1.2.0"
        metadata:
          datacenter: dc1
          zone: us-east-1a
      - id: web-2
        host: 192.168.1.11
        port: 8080
        weight: 2
        healthy: true
        tags:
          env: production
          version: "1.2.0"
        metadata:
          datacenter: dc1
          zone: us-east-1b

  api-service:
    instances:
      - id: api-1
        host: 10.0.1.100
        port: 9090
        weight: 1
        healthy: true
        tags:
          env: production
          service_type: api
        metadata:
          version: "2.1.0"
          database: postgres
      - id: api-2
        host: 10.0.1.101
        port: 9090
        weight: 1
        healthy: true
        tags:
          env: production
          service_type: api
        metadata:
          version: "2.1.0"
          database: postgres

  database-service:
    instances:
      - id: db-primary
        host: db-primary.internal
        port: 5432
        weight: 1
        priority: 0
        healthy: true
        tags:
          env: production
          role: primary
        metadata:
          version: "13.8"
          replication: master
      - id: db-replica
        host: db-replica.internal
        port: 5432
        weight: 1
        priority: 1
        healthy: true
        tags:
          env: production
          role: replica
        metadata:
          version: "13.8"
          replication: slave
`

func init() {
	// Create example services.yaml file if it doesn't exist
	if _, err := os.Stat("examples/services.yaml"); os.IsNotExist(err) {
		os.MkdirAll("examples", 0755)
		if err := os.WriteFile("examples/services.yaml", []byte(exampleServicesYAML), 0644); err != nil {
			log.Printf("Warning: Failed to create example services.yaml: %v", err)
		} else {
			log.Println("Created example services.yaml file")
		}
	}
}
