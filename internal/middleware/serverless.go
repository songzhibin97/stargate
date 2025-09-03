package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
)

// ServerlessMiddleware represents the serverless function integration middleware
type ServerlessMiddleware struct {
	config *config.ServerlessConfig
	client *http.Client
	mutex  sync.RWMutex

	// Statistics
	totalRequests       int64
	preProcessRequests  int64
	postProcessRequests int64
	failedRequests      int64
}

// ServerlessFunction represents a single serverless function configuration
type ServerlessFunction struct {
	ID          string            `yaml:"id" json:"id"`
	Name        string            `yaml:"name" json:"name"`
	URL         string            `yaml:"url" json:"url"`
	Method      string            `yaml:"method" json:"method"`
	Headers     map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	Timeout     time.Duration     `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	RetryCount  int               `yaml:"retry_count,omitempty" json:"retry_count,omitempty"`
	OnError     string            `yaml:"on_error,omitempty" json:"on_error,omitempty"` // continue, abort
}

// ServerlessRule represents a rule for when to execute serverless functions
type ServerlessRule struct {
	ID          string               `yaml:"id" json:"id"`
	Path        string               `yaml:"path" json:"path"`
	Method      string               `yaml:"method" json:"method"`
	Headers     map[string]string    `yaml:"headers,omitempty" json:"headers,omitempty"`
	PreProcess  []ServerlessFunction `yaml:"pre_process,omitempty" json:"pre_process,omitempty"`
	PostProcess []ServerlessFunction `yaml:"post_process,omitempty" json:"post_process,omitempty"`
}

// FunctionRequest represents the request payload sent to serverless function
type FunctionRequest struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
	Query   map[string]string `json:"query"`
}

// FunctionResponse represents the response from serverless function
type FunctionResponse struct {
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
	Status  int               `json:"status,omitempty"`
	Error   string            `json:"error,omitempty"`
}

// NewServerlessMiddleware creates a new serverless middleware
func NewServerlessMiddleware(cfg *config.ServerlessConfig) *ServerlessMiddleware {
	client := &http.Client{
		Timeout: cfg.DefaultTimeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     30 * time.Second,
		},
	}

	return &ServerlessMiddleware{
		config: cfg,
		client: client,
	}
}

// Handler returns the HTTP middleware handler
func (m *ServerlessMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip if middleware is disabled
			if !m.config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Update total requests statistics
			m.updateTotalRequests()

			// Find matching rule
			rule := m.matchRule(r)
			if rule == nil {
				// No matching rule, continue to next handler
				next.ServeHTTP(w, r)
				return
			}

			// Execute pre-process functions
			modifiedRequest, err := m.executePreProcessFunctions(r, rule)
			if err != nil {
				m.updateFailedRequests()
				m.handleError(w, r, http.StatusInternalServerError, fmt.Sprintf("Pre-process function failed: %v", err))
				return
			}

			// Create response wrapper to capture response for post-processing
			wrapper := &serverlessResponseWrapper{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				body:          &bytes.Buffer{},
			}

			// Continue to next handler with modified request
			next.ServeHTTP(wrapper, modifiedRequest)

			// Execute post-process functions
			err = m.executePostProcessFunctions(modifiedRequest, wrapper, rule)
			if err != nil {
				m.updateFailedRequests()
				// Use a fallback logger if middleware doesn't have one
				log.Printf("Post-process function failed: %v", err)
				// Don't return error for post-process failures, just log
			}
		})
	}
}

// matchRule finds a matching serverless rule for the request
func (m *ServerlessMiddleware) matchRule(r *http.Request) *ServerlessRule {
	path := r.URL.Path
	method := r.Method

	for _, rule := range m.config.Rules {
		// Match path and method
		if m.matchPath(path, rule.Path) && m.matchMethod(method, rule.Method) {
			// Check header matching if specified
			if len(rule.Headers) > 0 && !m.matchHeaders(r, rule.Headers) {
				continue
			}
			
			// Convert config rule to internal rule
			internalRule := &ServerlessRule{
				ID:          rule.ID,
				Path:        rule.Path,
				Method:      rule.Method,
				Headers:     rule.Headers,
				PreProcess:  make([]ServerlessFunction, len(rule.PreProcess)),
				PostProcess: make([]ServerlessFunction, len(rule.PostProcess)),
			}
			
			// Convert pre-process functions
			for i, fn := range rule.PreProcess {
				internalRule.PreProcess[i] = ServerlessFunction{
					ID:         fn.ID,
					Name:       fn.Name,
					URL:        fn.URL,
					Method:     fn.Method,
					Headers:    fn.Headers,
					Timeout:    fn.Timeout,
					RetryCount: fn.RetryCount,
					OnError:    fn.OnError,
				}
			}
			
			// Convert post-process functions
			for i, fn := range rule.PostProcess {
				internalRule.PostProcess[i] = ServerlessFunction{
					ID:         fn.ID,
					Name:       fn.Name,
					URL:        fn.URL,
					Method:     fn.Method,
					Headers:    fn.Headers,
					Timeout:    fn.Timeout,
					RetryCount: fn.RetryCount,
					OnError:    fn.OnError,
				}
			}
			
			return internalRule
		}
	}

	return nil
}

// matchPath checks if the request path matches the rule path
func (m *ServerlessMiddleware) matchPath(requestPath, rulePath string) bool {
	// Simple exact match for now, can be extended to support patterns
	return requestPath == rulePath
}

// matchMethod checks if the request method matches the rule method
func (m *ServerlessMiddleware) matchMethod(requestMethod, ruleMethod string) bool {
	// Empty rule method matches all methods
	if ruleMethod == "" {
		return true
	}
	return strings.EqualFold(requestMethod, ruleMethod)
}

// matchHeaders checks if request headers match rule headers
func (m *ServerlessMiddleware) matchHeaders(r *http.Request, ruleHeaders map[string]string) bool {
	for key, value := range ruleHeaders {
		if r.Header.Get(key) != value {
			return false
		}
	}
	return true
}

// executePreProcessFunctions executes pre-process serverless functions
func (m *ServerlessMiddleware) executePreProcessFunctions(r *http.Request, rule *ServerlessRule) (*http.Request, error) {
	if len(rule.PreProcess) == 0 {
		return r, nil
	}

	m.updatePreProcessRequests()

	// Read original request body
	var originalBody []byte
	if r.Body != nil {
		var err error
		originalBody, err = io.ReadAll(r.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
		// Restore body for further processing
		r.Body = io.NopCloser(bytes.NewReader(originalBody))
	}

	// Execute each pre-process function in sequence
	currentBody := string(originalBody)
	currentHeaders := make(map[string]string)
	for key, values := range r.Header {
		if len(values) > 0 {
			currentHeaders[key] = values[0]
		}
	}

	for _, function := range rule.PreProcess {
		response, err := m.callServerlessFunction(r, function, currentBody)
		if err != nil {
			if function.OnError == "continue" {
				log.Printf("Pre-process function %s failed but continuing: %v", function.Name, err)
				continue
			}
			return nil, fmt.Errorf("pre-process function %s failed: %w", function.Name, err)
		}

		// Apply function response to modify request
		if response.Body != "" {
			currentBody = response.Body
		}
		
		// Apply header modifications
		for key, value := range response.Headers {
			currentHeaders[key] = value
		}
	}

	// Create modified request
	modifiedRequest := r.Clone(r.Context())
	modifiedRequest.Body = io.NopCloser(strings.NewReader(currentBody))
	modifiedRequest.ContentLength = int64(len(currentBody))

	// Apply modified headers
	for key, value := range currentHeaders {
		modifiedRequest.Header.Set(key, value)
	}

	return modifiedRequest, nil
}

// executePostProcessFunctions executes post-process serverless functions
func (m *ServerlessMiddleware) executePostProcessFunctions(r *http.Request, wrapper *serverlessResponseWrapper, rule *ServerlessRule) error {
	if len(rule.PostProcess) == 0 {
		return nil
	}

	m.updatePostProcessRequests()

	// Execute each post-process function
	for _, function := range rule.PostProcess {
		// Create function request with response data
		functionReq := &FunctionRequest{
			Method:  r.Method,
			Path:    r.URL.Path,
			Headers: make(map[string]string),
			Body:    wrapper.body.String(),
			Query:   make(map[string]string),
		}

		// Add request headers
		for key, values := range r.Header {
			if len(values) > 0 {
				functionReq.Headers[key] = values[0]
			}
		}

		// Add query parameters
		for key, values := range r.URL.Query() {
			if len(values) > 0 {
				functionReq.Query[key] = values[0]
			}
		}

		_, err := m.callServerlessFunction(r, function, functionReq.Body)
		if err != nil {
			if function.OnError == "continue" {
				log.Printf("Post-process function %s failed but continuing: %v", function.Name, err)
				continue
			}
			return fmt.Errorf("post-process function %s failed: %w", function.Name, err)
		}
	}

	return nil
}

// callServerlessFunction calls a serverless function with retry logic
func (m *ServerlessMiddleware) callServerlessFunction(r *http.Request, function ServerlessFunction, body string) (*FunctionResponse, error) {
	// Prepare function request
	functionReq := &FunctionRequest{
		Method:  r.Method,
		Path:    r.URL.Path,
		Headers: make(map[string]string),
		Body:    body,
		Query:   make(map[string]string),
	}

	// Add request headers
	for key, values := range r.Header {
		if len(values) > 0 {
			functionReq.Headers[key] = values[0]
		}
	}

	// Add query parameters
	for key, values := range r.URL.Query() {
		if len(values) > 0 {
			functionReq.Query[key] = values[0]
		}
	}

	// Serialize request
	reqBody, err := json.Marshal(functionReq)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize function request: %w", err)
	}

	// Retry logic
	maxRetries := function.RetryCount
	if maxRetries <= 0 {
		maxRetries = 1
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		response, err := m.executeFunctionCall(function, reqBody)
		if err == nil {
			return response, nil
		}
		lastErr = err
		
		if attempt < maxRetries-1 {
			// Wait before retry
			time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
		}
	}

	return nil, fmt.Errorf("function call failed after %d attempts: %w", maxRetries, lastErr)
}

// executeFunctionCall executes a single function call
func (m *ServerlessMiddleware) executeFunctionCall(function ServerlessFunction, reqBody []byte) (*FunctionResponse, error) {
	// Set timeout
	timeout := function.Timeout
	if timeout == 0 {
		timeout = m.config.DefaultTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Create HTTP request
	method := function.Method
	if method == "" {
		method = "POST"
	}

	req, err := http.NewRequestWithContext(ctx, method, function.URL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	for key, value := range function.Headers {
		req.Header.Set(key, value)
	}

	// Execute request
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("function returned error status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var functionResp FunctionResponse
	if err := json.Unmarshal(respBody, &functionResp); err != nil {
		// If response is not JSON, treat as plain text body
		functionResp.Body = string(respBody)
		functionResp.Status = resp.StatusCode
	}

	return &functionResp, nil
}

// handleError handles error responses
func (m *ServerlessMiddleware) handleError(w http.ResponseWriter, r *http.Request, statusCode int, message string) {
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
func (m *ServerlessMiddleware) updateTotalRequests() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.totalRequests++
}

func (m *ServerlessMiddleware) updatePreProcessRequests() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.preProcessRequests++
}

func (m *ServerlessMiddleware) updatePostProcessRequests() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.postProcessRequests++
}

func (m *ServerlessMiddleware) updateFailedRequests() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.failedRequests++
}

// GetStats returns middleware statistics
func (m *ServerlessMiddleware) GetStats() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return map[string]interface{}{
		"total_requests":        m.totalRequests,
		"pre_process_requests":  m.preProcessRequests,
		"post_process_requests": m.postProcessRequests,
		"failed_requests":       m.failedRequests,
		"success_rate":          float64(m.totalRequests-m.failedRequests) / float64(m.totalRequests) * 100,
	}
}

// serverlessResponseWrapper wraps http.ResponseWriter to capture response data
type serverlessResponseWrapper struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (w *serverlessResponseWrapper) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *serverlessResponseWrapper) Write(data []byte) (int, error) {
	// Write to both the original response and our buffer
	w.body.Write(data)
	return w.ResponseWriter.Write(data)
}
