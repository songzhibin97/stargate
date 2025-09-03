package router

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/pkg/config"
	"gopkg.in/yaml.v3"
)

// Store represents a configuration store that uses config.Source interface
// to load and watch for routing configuration changes.
type Store struct {
	source       config.Source
	configMgr    *ConfigManager
	engine       *Engine
	mu           sync.RWMutex
	running      bool
	stopCh       chan struct{}
	wg           sync.WaitGroup
	lastUpdate   time.Time
	watchCtx     context.Context
	watchCancel  context.CancelFunc
}

// NewStore creates a new configuration store with the given config source.
//
// Parameters:
//   - source: The configuration source implementation (file, etcd, etc.)
//   - engine: The routing engine to update when configuration changes
//
// Returns:
//   - *Store: The configuration store instance
//   - error: Any error that occurred during initialization
func NewStore(source config.Source, engine *Engine) (*Store, error) {
	if source == nil {
		return nil, fmt.Errorf("config source cannot be nil")
	}

	if engine == nil {
		return nil, fmt.Errorf("routing engine cannot be nil")
	}

	return &Store{
		source:    source,
		configMgr: NewConfigManager(),
		engine:    engine,
		stopCh:    make(chan struct{}),
	}, nil
}

// Start starts the configuration store and begins watching for changes.
// It loads the initial configuration and sets up a watcher for updates.
func (s *Store) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("store is already running")
	}

	// Load initial configuration
	if err := s.loadConfiguration(); err != nil {
		return fmt.Errorf("failed to load initial configuration: %w", err)
	}

	// Apply initial configuration to the engine
	if err := s.engine.LoadFromConfigManager(s.configMgr); err != nil {
		return fmt.Errorf("failed to apply initial configuration to engine: %w", err)
	}

	// Set up watch context
	s.watchCtx, s.watchCancel = context.WithCancel(ctx)

	// Start watching for configuration changes
	s.wg.Add(1)
	go s.watchConfiguration()

	s.running = true
	s.lastUpdate = time.Now()

	log.Println("Configuration store started successfully")
	return nil
}

// Stop stops the configuration store and cleans up resources.
func (s *Store) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.running = false

	// Cancel watch context
	if s.watchCancel != nil {
		s.watchCancel()
	}

	// Signal stop to all goroutines
	close(s.stopCh)

	// Wait for all goroutines to finish
	s.wg.Wait()

	// Close the config source
	if err := s.source.Close(); err != nil {
		log.Printf("Error closing config source: %v", err)
	}

	log.Println("Configuration store stopped")
	return nil
}

// Reload manually reloads the configuration from the source.
func (s *Store) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("store is not running")
	}

	// Load configuration from source
	if err := s.loadConfiguration(); err != nil {
		return fmt.Errorf("failed to reload configuration: %w", err)
	}

	// Apply configuration to the engine
	if err := s.engine.LoadFromConfigManager(s.configMgr); err != nil {
		return fmt.Errorf("failed to apply reloaded configuration to engine: %w", err)
	}

	s.lastUpdate = time.Now()
	log.Println("Configuration reloaded successfully")
	return nil
}

// GetLastUpdate returns the timestamp of the last configuration update.
func (s *Store) GetLastUpdate() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastUpdate
}

// GetConfigManager returns the underlying configuration manager.
func (s *Store) GetConfigManager() *ConfigManager {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.configMgr
}

// IsRunning returns whether the store is currently running.
func (s *Store) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// loadConfiguration loads configuration data from the source and updates the config manager.
func (s *Store) loadConfiguration() error {
	// Get configuration data from source
	data, err := s.source.Get()
	if err != nil {
		return fmt.Errorf("failed to get configuration from source: %w", err)
	}

	// Load configuration into config manager
	if err := s.configMgr.LoadFromBytes(data); err != nil {
		return fmt.Errorf("failed to parse configuration data: %w", err)
	}

	return nil
}

// watchConfiguration watches for configuration changes and updates the engine accordingly.
func (s *Store) watchConfiguration() {
	defer s.wg.Done()

	// Start watching for changes
	ch, err := s.source.Watch(s.watchCtx)
	if err != nil {
		log.Printf("Failed to start watching configuration: %v", err)
		return
	}

	log.Println("Started watching for configuration changes")

	for {
		select {
		case <-s.stopCh:
			log.Println("Stopping configuration watcher")
			return

		case <-s.watchCtx.Done():
			log.Println("Configuration watch context cancelled")
			return

		case data, ok := <-ch:
			if !ok {
				log.Println("Configuration watch channel closed, attempting to restart...")
				// Try to restart the watcher
				time.Sleep(5 * time.Second)
				if s.restartWatcher() {
					continue
				}
				return
			}

			// Process configuration update
			if err := s.processConfigurationUpdate(data); err != nil {
				log.Printf("Failed to process configuration update: %v", err)
				continue
			}

			log.Println("Configuration updated successfully")
		}
	}
}

// processConfigurationUpdate processes a configuration update from the watch channel.
func (s *Store) processConfigurationUpdate(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("store is not running")
	}

	// Parse the new configuration
	var newConfig RoutingConfig
	if err := yaml.Unmarshal(data, &newConfig); err != nil {
		return fmt.Errorf("failed to parse configuration update: %w", err)
	}

	// Validate the new configuration
	if err := newConfig.Validate(); err != nil {
		return fmt.Errorf("invalid configuration update: %w", err)
	}

	// Update timestamps
	for i := range newConfig.Routes {
		newConfig.Routes[i].SetTimestamps()
	}
	for i := range newConfig.Upstreams {
		newConfig.Upstreams[i].SetTimestamps()
	}

	// Update the config manager
	s.configMgr.mu.Lock()
	s.configMgr.config = &newConfig
	s.configMgr.mu.Unlock()

	// Apply the new configuration to the engine
	if err := s.engine.LoadFromConfigManager(s.configMgr); err != nil {
		return fmt.Errorf("failed to apply configuration update to engine: %w", err)
	}

	s.lastUpdate = time.Now()
	return nil
}

// restartWatcher attempts to restart the configuration watcher.
func (s *Store) restartWatcher() bool {
	s.mu.RLock()
	running := s.running
	s.mu.RUnlock()

	if !running {
		return false
	}

	// Try to restart the watcher
	ch, err := s.source.Watch(s.watchCtx)
	if err != nil {
		log.Printf("Failed to restart configuration watcher: %v", err)
		return false
	}

	log.Println("Configuration watcher restarted successfully")

	// Continue processing updates
	go func() {
		for {
			select {
			case <-s.stopCh:
				return
			case <-s.watchCtx.Done():
				return
			case data, ok := <-ch:
				if !ok {
					return
				}
				if err := s.processConfigurationUpdate(data); err != nil {
					log.Printf("Failed to process configuration update: %v", err)
				}
			}
		}
	}()

	return true
}
