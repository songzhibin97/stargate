package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/store"
	"github.com/songzhibin97/stargate/pkg/log"
)

// ConfigNotifier handles configuration change notifications
type ConfigNotifier struct {
	config    *config.Config
	store     store.Store
	mu        sync.RWMutex
	listeners map[string][]ConfigChangeListener
	running   bool
	stopCh    chan struct{}
	wg        sync.WaitGroup
	logger    log.Logger
}

// ConfigChangeEvent represents a configuration change event
type ConfigChangeEvent struct {
	Type      ConfigChangeType `json:"type"`
	Key       string           `json:"key"`
	Value     []byte           `json:"value,omitempty"`
	OldValue  []byte           `json:"old_value,omitempty"`
	Timestamp int64            `json:"timestamp"`
	Version   string           `json:"version"`
	Source    string           `json:"source"` // "admin_api", "gitops", "sync"
}

// ConfigChangeType represents the type of configuration change
type ConfigChangeType string

const (
	ConfigChangeTypeCreate ConfigChangeType = "create"
	ConfigChangeTypeUpdate ConfigChangeType = "update"
	ConfigChangeTypeDelete ConfigChangeType = "delete"
)

// ConfigChangeListener is called when configuration changes
type ConfigChangeListener func(event *ConfigChangeEvent)

// NewConfigNotifier creates a new configuration notifier
func NewConfigNotifier(cfg *config.Config, store store.Store, logger log.Logger) *ConfigNotifier {
	if logger == nil {
		logger = log.Component("controller.config_notifier")
	}

	return &ConfigNotifier{
		config:    cfg,
		store:     store,
		listeners: make(map[string][]ConfigChangeListener),
		stopCh:    make(chan struct{}),
		logger:    logger.With(log.String("component", "config_notifier")),
	}
}

// Start starts the configuration notifier
func (cn *ConfigNotifier) Start() error {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	if cn.running {
		return fmt.Errorf("config notifier is already running")
	}

	cn.running = true

	// Start watching for configuration changes
	if err := cn.startWatching(); err != nil {
		cn.running = false
		return fmt.Errorf("failed to start watching: %w", err)
	}

	cn.logger.Info("Configuration notifier started")
	return nil
}

// Stop stops the configuration notifier
func (cn *ConfigNotifier) Stop() {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	if !cn.running {
		return
	}

	cn.running = false
	close(cn.stopCh)

	// Stop all watchers
	cn.store.Unwatch("routes/")
	cn.store.Unwatch("upstreams/")
	cn.store.Unwatch("plugins/")

	cn.wg.Wait()
	cn.logger.Info("Configuration notifier stopped")
}

// AddListener adds a configuration change listener
func (cn *ConfigNotifier) AddListener(pattern string, listener ConfigChangeListener) {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	cn.listeners[pattern] = append(cn.listeners[pattern], listener)
}

// RemoveListener removes a configuration change listener
func (cn *ConfigNotifier) RemoveListener(pattern string) {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	delete(cn.listeners, pattern)
}

// NotifyChange notifies listeners of a configuration change
func (cn *ConfigNotifier) NotifyChange(event *ConfigChangeEvent) {
	cn.mu.RLock()
	defer cn.mu.RUnlock()

	// Notify all matching listeners
	for pattern, listeners := range cn.listeners {
		if cn.matchesPattern(event.Key, pattern) {
			for _, listener := range listeners {
				go func(l ConfigChangeListener, e *ConfigChangeEvent) {
					defer func() {
						if r := recover(); r != nil {
							cn.logger.Error("Config listener panic",
								log.Any("panic", r),
							)
						}
					}()
					l(e)
				}(listener, event)
			}
		}
	}
}

// PublishConfigChange publishes a configuration change event
func (cn *ConfigNotifier) PublishConfigChange(changeType string, key string, value, oldValue []byte, source string) error {
	event := &ConfigChangeEvent{
		Type:      ConfigChangeType(changeType),
		Key:       key,
		Value:     value,
		OldValue:  oldValue,
		Timestamp: time.Now().Unix(),
		Version:   cn.generateVersion(),
		Source:    source,
	}

	// Store the change event for audit purposes
	if err := cn.storeChangeEvent(event); err != nil {
		cn.logger.Error("Failed to store change event",
			log.Error(err),
		)
	}

	// Notify listeners
	cn.NotifyChange(event)

	cn.logger.Info("Published config change",
		log.String("type", string(changeType)),
		log.String("key", key),
		log.String("source", source),
	)
	return nil
}

// startWatching starts watching for configuration changes
func (cn *ConfigNotifier) startWatching() error {
	// Watch routes
	if err := cn.store.Watch("routes/", cn.onConfigChange); err != nil {
		return fmt.Errorf("failed to watch routes: %w", err)
	}

	// Watch upstreams
	if err := cn.store.Watch("upstreams/", cn.onConfigChange); err != nil {
		return fmt.Errorf("failed to watch upstreams: %w", err)
	}

	// Watch plugins
	if err := cn.store.Watch("plugins/", cn.onConfigChange); err != nil {
		return fmt.Errorf("failed to watch plugins: %w", err)
	}

	return nil
}

// onConfigChange handles configuration changes from etcd
func (cn *ConfigNotifier) onConfigChange(key string, value []byte, eventType store.EventType) {
	var changeType ConfigChangeType
	switch eventType {
	case store.EventTypePut:
		// Check if this is a create or update
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		if _, err := cn.store.Get(ctx, key); err != nil {
			changeType = ConfigChangeTypeCreate
		} else {
			changeType = ConfigChangeTypeUpdate
		}
	case store.EventTypeDelete:
		changeType = ConfigChangeTypeDelete
	default:
		return
	}

	// Create and publish change event
	event := &ConfigChangeEvent{
		Type:      changeType,
		Key:       key,
		Value:     value,
		Timestamp: time.Now().Unix(),
		Version:   cn.generateVersion(),
		Source:    "etcd_watch",
	}

	cn.NotifyChange(event)
}

// storeChangeEvent stores a change event for audit purposes
func (cn *ConfigNotifier) storeChangeEvent(event *ConfigChangeEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	eventKey := fmt.Sprintf("events/config_changes/%d_%s", event.Timestamp, event.Key)
	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	return cn.store.Put(ctx, eventKey, eventData)
}

// generateVersion generates a version string for the configuration change
func (cn *ConfigNotifier) generateVersion() string {
	return fmt.Sprintf("v%d", time.Now().UnixNano())
}

// matchesPattern checks if a key matches a pattern
func (cn *ConfigNotifier) matchesPattern(key, pattern string) bool {
	// Simple pattern matching - can be enhanced with glob patterns
	if pattern == "*" {
		return true
	}
	
	// Check if key starts with pattern
	if len(key) >= len(pattern) && key[:len(pattern)] == pattern {
		return true
	}
	
	return false
}

// GetChangeHistory returns the history of configuration changes
func (cn *ConfigNotifier) GetChangeHistory(limit int) ([]*ConfigChangeEvent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	eventsData, err := cn.store.List(ctx, "events/config_changes/")
	if err != nil {
		return nil, fmt.Errorf("failed to list change events: %w", err)
	}

	var events []*ConfigChangeEvent
	for _, data := range eventsData {
		var event ConfigChangeEvent
		if err := json.Unmarshal(data, &event); err != nil {
			continue // Skip invalid events
		}
		events = append(events, &event)
	}

	// Sort by timestamp (newest first) and limit
	if len(events) > limit && limit > 0 {
		events = events[:limit]
	}

	return events, nil
}

// Health returns the health status of the config notifier
func (cn *ConfigNotifier) Health() map[string]interface{} {
	cn.mu.RLock()
	defer cn.mu.RUnlock()

	return map[string]interface{}{
		"status":           "healthy",
		"running":          cn.running,
		"listeners_count":  len(cn.listeners),
		"watchers_active":  cn.running,
	}
}

// Metrics returns metrics for the config notifier
func (cn *ConfigNotifier) Metrics() map[string]interface{} {
	cn.mu.RLock()
	defer cn.mu.RUnlock()

	return map[string]interface{}{
		"running":          cn.running,
		"listeners_count":  len(cn.listeners),
		"watchers_active":  cn.running,
	}
}
