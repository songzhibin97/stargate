package file

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/pkg/config"
)

// FileSource implements the config.Source interface for file-based configuration.
// It provides configuration loading from local files and watches for file changes.
type FileSource struct {
	filePath     string
	pollInterval time.Duration
	mu           sync.RWMutex
	lastModTime  time.Time
	watchers     map[string]*watcher
	stopCh       chan struct{}
	wg           sync.WaitGroup
}

// watcher represents a file watcher instance
type watcher struct {
	ctx    context.Context
	cancel context.CancelFunc
	ch     chan []byte
}

// NewFileSource creates a new file-based configuration source.
//
// Parameters:
//   - filePath: Path to the configuration file
//   - pollInterval: Interval for checking file modifications (default: 1 second)
//
// Returns:
//   - config.Source: The file source implementation
//   - error: Any error that occurred during initialization
func NewFileSource(filePath string, pollInterval time.Duration) (config.Source, error) {
	if filePath == "" {
		return nil, fmt.Errorf("file path cannot be empty")
	}

	// Check if file exists and is readable
	if _, err := os.Stat(filePath); err != nil {
		return nil, fmt.Errorf("failed to access file %s: %w", filePath, err)
	}

	if pollInterval <= 0 {
		pollInterval = time.Second // Default to 1 second
	}

	fs := &FileSource{
		filePath:     filePath,
		pollInterval: pollInterval,
		watchers:     make(map[string]*watcher),
		stopCh:       make(chan struct{}),
	}

	// Get initial modification time
	if stat, err := os.Stat(filePath); err == nil {
		fs.lastModTime = stat.ModTime()
	}

	return fs, nil
}

// Get retrieves the complete configuration data from the file.
// It reads the entire file content and returns it as bytes.
func (fs *FileSource) Get() ([]byte, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	data, err := os.ReadFile(fs.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", fs.filePath, err)
	}

	return data, nil
}

// Watch monitors the configuration file for changes and returns a channel
// that delivers the latest complete configuration data whenever changes occur.
//
// The implementation:
//   - Sends the current configuration immediately upon successful setup
//   - Polls the file modification time at regular intervals
//   - Sends updated configuration when file changes are detected
//   - Closes the channel when the context is cancelled
func (fs *FileSource) Watch(ctx context.Context) (<-chan []byte, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Create a new watcher
	watcherCtx, cancel := context.WithCancel(ctx)
	ch := make(chan []byte, 1)

	w := &watcher{
		ctx:    watcherCtx,
		cancel: cancel,
		ch:     ch,
	}

	// Generate a unique key for this watcher
	watcherKey := fmt.Sprintf("watcher_%d", time.Now().UnixNano())
	fs.watchers[watcherKey] = w

	// Send initial configuration
	go func() {
		defer func() {
			fs.mu.Lock()
			delete(fs.watchers, watcherKey)
			fs.mu.Unlock()
			close(ch)
		}()

		// Send current config immediately
		if data, err := fs.readFile(); err == nil {
			select {
			case ch <- data:
			case <-watcherCtx.Done():
				return
			}
		}

		// Start polling for changes
		ticker := time.NewTicker(fs.pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-watcherCtx.Done():
				return
			case <-ticker.C:
				if fs.checkAndNotifyChanges(ch, watcherCtx) {
					// File changed, data sent
				}
			}
		}
	}()

	return ch, nil
}

// readFile reads the configuration file and returns its content
func (fs *FileSource) readFile() ([]byte, error) {
	return os.ReadFile(fs.filePath)
}

// checkAndNotifyChanges checks if the file has been modified and sends
// the new content to the channel if it has changed
func (fs *FileSource) checkAndNotifyChanges(ch chan []byte, ctx context.Context) bool {
	stat, err := os.Stat(fs.filePath)
	if err != nil {
		// File might have been deleted or become inaccessible
		return false
	}

	fs.mu.Lock()
	lastModTime := fs.lastModTime
	fs.mu.Unlock()

	// Check if file has been modified
	if stat.ModTime().After(lastModTime) {
		// File has been modified, read new content
		data, err := fs.readFile()
		if err != nil {
			return false
		}

		// Update last modification time
		fs.mu.Lock()
		fs.lastModTime = stat.ModTime()
		fs.mu.Unlock()

		// Send new data to channel
		select {
		case ch <- data:
			return true
		case <-ctx.Done():
			return false
		}
	}

	return false
}

// Close stops all watchers and cleans up resources
func (fs *FileSource) Close() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Cancel all watchers
	for _, w := range fs.watchers {
		w.cancel()
	}

	// Clear watchers map
	fs.watchers = make(map[string]*watcher)

	return nil
}
