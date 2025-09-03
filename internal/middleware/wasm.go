package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// WASMMiddleware represents the WASM plugin middleware
type WASMMiddleware struct {
	config  *config.WASMConfig
	runtime wazero.Runtime
	plugins map[string]*WASMPlugin
	mutex   sync.RWMutex

	// Statistics
	totalRequests   int64
	pluginRequests  int64
	failedRequests  int64
	pluginLoadTime  time.Duration
}

// WASMPlugin represents a loaded WASM plugin
type WASMPlugin struct {
	ID          string
	Name        string
	Path        string
	Module      api.Module
	LoadedAt    time.Time
	LastUsed    time.Time
	CallCount   int64
	ErrorCount  int64
}

// PluginRequest represents the request data passed to WASM plugin
type PluginRequest struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
	Query   map[string]string `json:"query"`
}

// PluginResponse represents the response from WASM plugin
type PluginResponse struct {
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
	Status  int               `json:"status,omitempty"`
	Error   string            `json:"error,omitempty"`
	Continue bool             `json:"continue,omitempty"`
}

// NewWASMMiddleware creates a new WASM middleware
func NewWASMMiddleware(cfg *config.WASMConfig) (*WASMMiddleware, error) {
	// Create wazero runtime
	ctx := context.Background()
	runtime := wazero.NewRuntime(ctx)

	// Instantiate WASI
	_, err := wasi_snapshot_preview1.Instantiate(ctx, runtime)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate WASI: %w", err)
	}

	middleware := &WASMMiddleware{
		config:  cfg,
		runtime: runtime,
		plugins: make(map[string]*WASMPlugin),
	}

	// Load plugins
	if err := middleware.loadPlugins(ctx); err != nil {
		return nil, fmt.Errorf("failed to load plugins: %w", err)
	}

	return middleware, nil
}

// loadPlugins loads all configured WASM plugins
func (m *WASMMiddleware) loadPlugins(ctx context.Context) error {
	startTime := time.Now()
	defer func() {
		m.pluginLoadTime = time.Since(startTime)
	}()

	for _, pluginConfig := range m.config.Plugins {
		if err := m.loadPlugin(ctx, pluginConfig); err != nil {
			log.Printf("Failed to load plugin %s: %v", pluginConfig.Name, err)
			if pluginConfig.Required {
				return fmt.Errorf("required plugin %s failed to load: %w", pluginConfig.Name, err)
			}
		}
	}

	return nil
}

// loadPlugin loads a single WASM plugin
func (m *WASMMiddleware) loadPlugin(ctx context.Context, pluginConfig config.WASMPlugin) error {
	// Read WASM file
	wasmBytes, err := os.ReadFile(pluginConfig.Path)
	if err != nil {
		return fmt.Errorf("failed to read WASM file %s: %w", pluginConfig.Path, err)
	}

	// Compile module
	compiledModule, err := m.runtime.CompileModule(ctx, wasmBytes)
	if err != nil {
		return fmt.Errorf("failed to compile WASM module: %w", err)
	}

	// Create host functions for the plugin
	hostModule := m.runtime.NewHostModuleBuilder("env")
	
	// Add host functions that plugins can call
	hostModule.NewFunctionBuilder().
		WithName("log").
		WithParameterNames("ptr", "len").
		WithFunc(m.hostLog).
		Export("log")

	hostModule.NewFunctionBuilder().
		WithName("get_header").
		WithParameterNames("key_ptr", "key_len", "value_ptr", "value_len").
		WithFunc(m.hostGetHeader).
		Export("get_header")

	hostModule.NewFunctionBuilder().
		WithName("set_header").
		WithParameterNames("key_ptr", "key_len", "value_ptr", "value_len").
		WithFunc(m.hostSetHeader).
		Export("set_header")

	// Instantiate host module
	_, err = hostModule.Instantiate(ctx)
	if err != nil {
		return fmt.Errorf("failed to instantiate host module: %w", err)
	}

	// Instantiate the plugin module
	moduleConfig := wazero.NewModuleConfig().WithName(pluginConfig.Name)
	module, err := m.runtime.InstantiateModule(ctx, compiledModule, moduleConfig)
	if err != nil {
		return fmt.Errorf("failed to instantiate module: %w", err)
	}

	// Create plugin instance
	plugin := &WASMPlugin{
		ID:       pluginConfig.ID,
		Name:     pluginConfig.Name,
		Path:     pluginConfig.Path,
		Module:   module,
		LoadedAt: time.Now(),
		LastUsed: time.Now(),
	}

	m.mutex.Lock()
	m.plugins[pluginConfig.ID] = plugin
	m.mutex.Unlock()

	log.Printf("Successfully loaded WASM plugin: %s", pluginConfig.Name)
	return nil
}

// Handler returns the HTTP middleware handler
func (m *WASMMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip if middleware is disabled
			if !m.config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Update total requests statistics
			m.updateTotalRequests()

			// Find matching plugins for this request
			matchingPlugins := m.findMatchingPlugins(r)
			if len(matchingPlugins) == 0 {
				// No matching plugins, continue to next handler
				next.ServeHTTP(w, r)
				return
			}

			// Execute plugins
			modifiedRequest, shouldContinue, err := m.executePlugins(r, matchingPlugins)
			if err != nil {
				m.updateFailedRequests()
				m.handleError(w, r, http.StatusInternalServerError, fmt.Sprintf("Plugin execution failed: %v", err))
				return
			}

			if !shouldContinue {
				// Plugin decided to stop processing
				return
			}

			// Continue to next handler with modified request
			next.ServeHTTP(w, modifiedRequest)
		})
	}
}

// findMatchingPlugins finds plugins that match the current request
func (m *WASMMiddleware) findMatchingPlugins(r *http.Request) []*WASMPlugin {
	var matchingPlugins []*WASMPlugin

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for _, rule := range m.config.Rules {
		if m.matchRule(r, rule) {
			for _, pluginID := range rule.Plugins {
				if plugin, exists := m.plugins[pluginID]; exists {
					matchingPlugins = append(matchingPlugins, plugin)
				}
			}
		}
	}

	return matchingPlugins
}

// matchRule checks if a request matches a plugin rule
func (m *WASMMiddleware) matchRule(r *http.Request, rule config.WASMRule) bool {
	// Match path
	if rule.Path != "" && !m.matchPath(r.URL.Path, rule.Path) {
		return false
	}

	// Match method
	if rule.Method != "" && !strings.EqualFold(r.Method, rule.Method) {
		return false
	}

	// Match headers
	for key, value := range rule.Headers {
		if r.Header.Get(key) != value {
			return false
		}
	}

	return true
}

// matchPath checks if the request path matches the rule path
func (m *WASMMiddleware) matchPath(requestPath, rulePath string) bool {
	// Simple exact match for now, can be extended to support patterns
	return requestPath == rulePath
}

// executePlugins executes matching WASM plugins
func (m *WASMMiddleware) executePlugins(r *http.Request, plugins []*WASMPlugin) (*http.Request, bool, error) {
	m.updatePluginRequests()

	// Read request body
	var requestBody []byte
	if r.Body != nil {
		var err error
		requestBody, err = io.ReadAll(r.Body)
		if err != nil {
			return nil, false, fmt.Errorf("failed to read request body: %w", err)
		}
		// Restore body for further processing
		r.Body = io.NopCloser(strings.NewReader(string(requestBody)))
	}

	// Prepare plugin request
	pluginReq := &PluginRequest{
		Method:  r.Method,
		Path:    r.URL.Path,
		Headers: make(map[string]string),
		Body:    string(requestBody),
		Query:   make(map[string]string),
	}

	// Add headers
	for key, values := range r.Header {
		if len(values) > 0 {
			pluginReq.Headers[key] = values[0]
		}
	}

	// Add query parameters
	for key, values := range r.URL.Query() {
		if len(values) > 0 {
			pluginReq.Query[key] = values[0]
		}
	}

	// Execute each plugin in sequence
	currentRequest := pluginReq
	for _, plugin := range plugins {
		response, err := m.executePlugin(plugin, currentRequest)
		if err != nil {
			plugin.ErrorCount++
			return nil, false, fmt.Errorf("plugin %s execution failed: %w", plugin.Name, err)
		}

		plugin.CallCount++
		plugin.LastUsed = time.Now()

		// Check if plugin wants to stop processing
		if !response.Continue {
			return nil, false, nil
		}

		// Apply plugin modifications
		if response.Body != "" {
			currentRequest.Body = response.Body
		}

		// Apply header modifications
		for key, value := range response.Headers {
			currentRequest.Headers[key] = value
		}
	}

	// Create modified HTTP request
	modifiedRequest := r.Clone(r.Context())
	modifiedRequest.Body = io.NopCloser(strings.NewReader(currentRequest.Body))
	modifiedRequest.ContentLength = int64(len(currentRequest.Body))

	// Apply modified headers
	for key, value := range currentRequest.Headers {
		modifiedRequest.Header.Set(key, value)
	}

	return modifiedRequest, true, nil
}

// executePlugin executes a single WASM plugin
func (m *WASMMiddleware) executePlugin(plugin *WASMPlugin, request *PluginRequest) (*PluginResponse, error) {
	ctx := context.Background()

	// Serialize request to JSON
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize request: %w", err)
	}

	// Get plugin's process_request function
	processRequestFunc := plugin.Module.ExportedFunction("process_request")
	if processRequestFunc == nil {
		return nil, fmt.Errorf("plugin does not export process_request function")
	}

	// Allocate memory in WASM module for request data
	mallocFunc := plugin.Module.ExportedFunction("malloc")
	if mallocFunc == nil {
		return nil, fmt.Errorf("plugin does not export malloc function")
	}

	// Allocate memory for request
	requestSize := uint64(len(requestJSON))
	results, err := mallocFunc.Call(ctx, requestSize)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate memory: %w", err)
	}
	requestPtr := results[0]

	// Write request data to WASM memory
	if !plugin.Module.Memory().Write(uint32(requestPtr), requestJSON) {
		return nil, fmt.Errorf("failed to write request data to WASM memory")
	}

	// Call the plugin function
	results, err = processRequestFunc.Call(ctx, requestPtr, requestSize)
	if err != nil {
		return nil, fmt.Errorf("plugin function call failed: %w", err)
	}

	// Get response pointer and size
	responsePtr := results[0]
	responseSize := results[1]

	// Read response from WASM memory
	responseData, ok := plugin.Module.Memory().Read(uint32(responsePtr), uint32(responseSize))
	if !ok {
		return nil, fmt.Errorf("failed to read response from WASM memory")
	}

	// Parse response
	var response PluginResponse
	if err := json.Unmarshal(responseData, &response); err != nil {
		return nil, fmt.Errorf("failed to parse plugin response: %w", err)
	}

	// Free allocated memory
	freeFunc := plugin.Module.ExportedFunction("free")
	if freeFunc != nil {
		freeFunc.Call(ctx, requestPtr)
		freeFunc.Call(ctx, responsePtr)
	}

	return &response, nil
}

// Host functions that plugins can call

// hostLog allows plugins to log messages
func (m *WASMMiddleware) hostLog(ctx context.Context, mod api.Module, ptr, len uint32) {
	data, ok := mod.Memory().Read(ptr, len)
	if !ok {
		log.Printf("WASM Plugin Log: failed to read log message")
		return
	}
	log.Printf("WASM Plugin Log: %s", string(data))
}

// hostGetHeader allows plugins to get request headers
func (m *WASMMiddleware) hostGetHeader(ctx context.Context, mod api.Module, keyPtr, keyLen, valuePtr, valueLen uint32) uint32 {
	// This is a simplified implementation
	// In a real implementation, you'd need to maintain request context
	return 0
}

// hostSetHeader allows plugins to set response headers
func (m *WASMMiddleware) hostSetHeader(ctx context.Context, mod api.Module, keyPtr, keyLen, valuePtr, valueLen uint32) {
	// This is a simplified implementation
	// In a real implementation, you'd need to maintain response context
}

// handleError handles error responses
func (m *WASMMiddleware) handleError(w http.ResponseWriter, r *http.Request, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResponse := map[string]interface{}{
		"error":   message,
		"status":  statusCode,
		"path":    r.URL.Path,
		"method":  r.Method,
	}

	json.NewEncoder(w).Encode(errorResponse)
}

// Statistics methods
func (m *WASMMiddleware) updateTotalRequests() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.totalRequests++
}

func (m *WASMMiddleware) updatePluginRequests() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.pluginRequests++
}

func (m *WASMMiddleware) updateFailedRequests() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.failedRequests++
}

// GetStats returns middleware statistics
func (m *WASMMiddleware) GetStats() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	pluginStats := make(map[string]interface{})
	for id, plugin := range m.plugins {
		pluginStats[id] = map[string]interface{}{
			"name":        plugin.Name,
			"loaded_at":   plugin.LoadedAt,
			"last_used":   plugin.LastUsed,
			"call_count":  plugin.CallCount,
			"error_count": plugin.ErrorCount,
		}
	}

	return map[string]interface{}{
		"total_requests":    m.totalRequests,
		"plugin_requests":   m.pluginRequests,
		"failed_requests":   m.failedRequests,
		"success_rate":      float64(m.pluginRequests-m.failedRequests) / float64(m.pluginRequests) * 100,
		"plugin_load_time":  m.pluginLoadTime.Milliseconds(),
		"loaded_plugins":    len(m.plugins),
		"plugin_stats":      pluginStats,
	}
}

// Close closes the WASM runtime and cleans up resources
func (m *WASMMiddleware) Close(ctx context.Context) error {
	return m.runtime.Close(ctx)
}
