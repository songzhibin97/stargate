package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/store"
)

// VersionManager manages configuration versions and provides rollback capabilities
type VersionManager struct {
	config  *config.Config
	store   store.Store
	mu      sync.RWMutex
	current *ConfigVersion
}

// ConfigVersion represents a configuration version
type ConfigVersion struct {
	ID          string                 `json:"id"`
	Version     string                 `json:"version"`
	Description string                 `json:"description"`
	Timestamp   int64                  `json:"timestamp"`
	Author      string                 `json:"author"`
	Changes     []ConfigChange         `json:"changes"`
	Snapshot    map[string]interface{} `json:"snapshot"`
	Status      VersionStatus          `json:"status"`
	Metadata    map[string]string      `json:"metadata,omitempty"`
}

// ConfigChange represents a single configuration change
type ConfigChange struct {
	Type        string      `json:"type"`        // "create", "update", "delete"
	Resource    string      `json:"resource"`    // "route", "upstream", "plugin"
	ResourceID  string      `json:"resource_id"`
	OldValue    interface{} `json:"old_value,omitempty"`
	NewValue    interface{} `json:"new_value,omitempty"`
	Description string      `json:"description"`
}

// VersionStatus represents the status of a configuration version
type VersionStatus string

const (
	VersionStatusDraft    VersionStatus = "draft"
	VersionStatusActive   VersionStatus = "active"
	VersionStatusRolledBack VersionStatus = "rolled_back"
	VersionStatusArchived VersionStatus = "archived"
)

// NewVersionManager creates a new version manager
func NewVersionManager(cfg *config.Config, store store.Store) *VersionManager {
	return &VersionManager{
		config: cfg,
		store:  store,
	}
}

// CreateVersion creates a new configuration version
func (vm *VersionManager) CreateVersion(description, author string, changes []ConfigChange) (*ConfigVersion, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	// Generate version ID and number
	versionID := vm.generateVersionID()
	versionNumber := vm.generateVersionNumber()

	// Create configuration snapshot
	snapshot, err := vm.createSnapshot()
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot: %w", err)
	}

	version := &ConfigVersion{
		ID:          versionID,
		Version:     versionNumber,
		Description: description,
		Timestamp:   time.Now().Unix(),
		Author:      author,
		Changes:     changes,
		Snapshot:    snapshot,
		Status:      VersionStatusDraft,
		Metadata:    make(map[string]string),
	}

	// Store the version
	if err := vm.storeVersion(version); err != nil {
		return nil, fmt.Errorf("failed to store version: %w", err)
	}

	log.Printf("Created configuration version: %s (%s)", version.Version, version.ID)
	return version, nil
}

// ActivateVersion activates a configuration version
func (vm *VersionManager) ActivateVersion(versionID string) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	// Get the version to activate
	version, err := vm.getVersion(versionID)
	if err != nil {
		return fmt.Errorf("failed to get version: %w", err)
	}

	// Deactivate current version if exists
	if vm.current != nil {
		vm.current.Status = VersionStatusArchived
		if err := vm.storeVersion(vm.current); err != nil {
			log.Printf("Failed to archive current version: %v", err)
		}
	}

	// Activate the new version
	version.Status = VersionStatusActive
	if err := vm.storeVersion(version); err != nil {
		return fmt.Errorf("failed to store activated version: %w", err)
	}

	vm.current = version

	// Apply the configuration snapshot
	if err := vm.applySnapshot(version.Snapshot); err != nil {
		return fmt.Errorf("failed to apply snapshot: %w", err)
	}

	log.Printf("Activated configuration version: %s (%s)", version.Version, version.ID)
	return nil
}

// RollbackToVersion rolls back to a previous configuration version
func (vm *VersionManager) RollbackToVersion(versionID string) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	// Get the version to rollback to
	version, err := vm.getVersion(versionID)
	if err != nil {
		return fmt.Errorf("failed to get version: %w", err)
	}

	// Mark current version as rolled back
	if vm.current != nil {
		vm.current.Status = VersionStatusRolledBack
		if err := vm.storeVersion(vm.current); err != nil {
			log.Printf("Failed to mark current version as rolled back: %v", err)
		}
	}

	// Create a new version for the rollback
	rollbackVersion := &ConfigVersion{
		ID:          vm.generateVersionID(),
		Version:     vm.generateVersionNumber(),
		Description: fmt.Sprintf("Rollback to version %s", version.Version),
		Timestamp:   time.Now().Unix(),
		Author:      "system",
		Changes:     []ConfigChange{},
		Snapshot:    version.Snapshot, // Use the same snapshot
		Status:      VersionStatusActive,
		Metadata: map[string]string{
			"rollback_from": vm.current.ID,
			"rollback_to":   version.ID,
		},
	}

	// Store the rollback version
	if err := vm.storeVersion(rollbackVersion); err != nil {
		return fmt.Errorf("failed to store rollback version: %w", err)
	}

	vm.current = rollbackVersion

	// Apply the rollback snapshot
	if err := vm.applySnapshot(rollbackVersion.Snapshot); err != nil {
		return fmt.Errorf("failed to apply rollback snapshot: %w", err)
	}

	log.Printf("Rolled back to configuration version: %s (%s)", version.Version, version.ID)
	return nil
}

// GetCurrentVersion returns the current active version
func (vm *VersionManager) GetCurrentVersion() *ConfigVersion {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	return vm.current
}

// ListVersions returns all configuration versions
func (vm *VersionManager) ListVersions(limit int) ([]*ConfigVersion, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	versionsData, err := vm.store.List(ctx, "versions/")
	if err != nil {
		return nil, fmt.Errorf("failed to list versions: %w", err)
	}

	var versions []*ConfigVersion
	for _, data := range versionsData {
		var version ConfigVersion
		if err := json.Unmarshal(data, &version); err != nil {
			log.Printf("Failed to unmarshal version: %v", err)
			continue
		}
		versions = append(versions, &version)
	}

	// Sort by timestamp (newest first)
	for i := 0; i < len(versions)-1; i++ {
		for j := i + 1; j < len(versions); j++ {
			if versions[i].Timestamp < versions[j].Timestamp {
				versions[i], versions[j] = versions[j], versions[i]
			}
		}
	}

	// Apply limit
	if limit > 0 && len(versions) > limit {
		versions = versions[:limit]
	}

	return versions, nil
}

// createSnapshot creates a snapshot of the current configuration
func (vm *VersionManager) createSnapshot() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	snapshot := make(map[string]interface{})

	// Snapshot routes
	routesData, err := vm.store.List(ctx, "routes/")
	if err != nil {
		return nil, fmt.Errorf("failed to snapshot routes: %w", err)
	}
	snapshot["routes"] = routesData

	// Snapshot upstreams
	upstreamsData, err := vm.store.List(ctx, "upstreams/")
	if err != nil {
		return nil, fmt.Errorf("failed to snapshot upstreams: %w", err)
	}
	snapshot["upstreams"] = upstreamsData

	// Snapshot plugins
	pluginsData, err := vm.store.List(ctx, "plugins/")
	if err != nil {
		return nil, fmt.Errorf("failed to snapshot plugins: %w", err)
	}
	snapshot["plugins"] = pluginsData

	return snapshot, nil
}

// applySnapshot applies a configuration snapshot
func (vm *VersionManager) applySnapshot(snapshot map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Apply routes
	if routesData, ok := snapshot["routes"].(map[string][]byte); ok {
		// Clear existing routes
		if existingRoutes, err := vm.store.List(ctx, "routes/"); err == nil {
			for key := range existingRoutes {
				vm.store.Delete(ctx, key)
			}
		}

		// Apply snapshot routes
		for key, data := range routesData {
			if err := vm.store.Put(ctx, key, data); err != nil {
				log.Printf("Failed to apply route %s: %v", key, err)
			}
		}
	}

	// Apply upstreams
	if upstreamsData, ok := snapshot["upstreams"].(map[string][]byte); ok {
		// Clear existing upstreams
		if existingUpstreams, err := vm.store.List(ctx, "upstreams/"); err == nil {
			for key := range existingUpstreams {
				vm.store.Delete(ctx, key)
			}
		}

		// Apply snapshot upstreams
		for key, data := range upstreamsData {
			if err := vm.store.Put(ctx, key, data); err != nil {
				log.Printf("Failed to apply upstream %s: %v", key, err)
			}
		}
	}

	// Apply plugins
	if pluginsData, ok := snapshot["plugins"].(map[string][]byte); ok {
		// Clear existing plugins
		if existingPlugins, err := vm.store.List(ctx, "plugins/"); err == nil {
			for key := range existingPlugins {
				vm.store.Delete(ctx, key)
			}
		}

		// Apply snapshot plugins
		for key, data := range pluginsData {
			if err := vm.store.Put(ctx, key, data); err != nil {
				log.Printf("Failed to apply plugin %s: %v", key, err)
			}
		}
	}

	return nil
}

// storeVersion stores a configuration version
func (vm *VersionManager) storeVersion(version *ConfigVersion) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := fmt.Sprintf("versions/%s", version.ID)
	data, err := json.Marshal(version)
	if err != nil {
		return fmt.Errorf("failed to marshal version: %w", err)
	}

	return vm.store.Put(ctx, key, data)
}

// getVersion retrieves a configuration version by ID
func (vm *VersionManager) getVersion(versionID string) (*ConfigVersion, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := fmt.Sprintf("versions/%s", versionID)
	data, err := vm.store.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get version: %w", err)
	}

	var version ConfigVersion
	if err := json.Unmarshal(data, &version); err != nil {
		return nil, fmt.Errorf("failed to unmarshal version: %w", err)
	}

	return &version, nil
}

// generateVersionID generates a unique version ID
func (vm *VersionManager) generateVersionID() string {
	return fmt.Sprintf("version-%d", time.Now().UnixNano())
}

// generateVersionNumber generates a version number
func (vm *VersionManager) generateVersionNumber() string {
	return fmt.Sprintf("v%d", time.Now().Unix())
}

// Health returns the health status of the version manager
func (vm *VersionManager) Health() map[string]interface{} {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	health := map[string]interface{}{
		"current_version": nil,
	}

	if vm.current != nil {
		health["current_version"] = map[string]interface{}{
			"id":        vm.current.ID,
			"version":   vm.current.Version,
			"timestamp": vm.current.Timestamp,
			"status":    vm.current.Status,
		}
	}

	return health
}
