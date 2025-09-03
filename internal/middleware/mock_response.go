package middleware

import (
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
)

// MockResponseMiddleware handles mock responses
type MockResponseMiddleware struct {
	config       *config.MockResponseConfig
	compiledRules []*compiledMockRule
	mu           sync.RWMutex
	stats        *MockResponseStats
}

// compiledMockRule represents a compiled mock rule with pre-compiled regex patterns
type compiledMockRule struct {
	rule        *config.MockRule
	pathRegexes []*regexp.Regexp
}

// MockResponseStats represents statistics for mock responses
type MockResponseStats struct {
	RequestsProcessed int64     `json:"requests_processed"`
	MocksMatched      int64     `json:"mocks_matched"`
	MocksServed       int64     `json:"mocks_served"`
	RuleHits          map[string]int64 `json:"rule_hits"`
	LastProcessedAt   time.Time `json:"last_processed_at"`
	LastMatchedAt     time.Time `json:"last_matched_at"`
}

// MockMatchResult represents the result of mock rule matching
type MockMatchResult struct {
	Matched   bool
	Rule      *config.MockRule
	RuleID    string
	MatchedAt time.Time
}

// NewMockResponseMiddleware creates a new mock response middleware
func NewMockResponseMiddleware(config *config.MockResponseConfig) (*MockResponseMiddleware, error) {
	middleware := &MockResponseMiddleware{
		config: config,
		stats: &MockResponseStats{
			RuleHits:        make(map[string]int64),
			LastProcessedAt: time.Now(),
		},
	}

	// Compile rules
	if err := middleware.compileRules(); err != nil {
		return nil, err
	}

	return middleware, nil
}

// Handler returns the HTTP middleware handler
func (m *MockResponseMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip if middleware is disabled
			if !m.config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Update statistics
			m.updateProcessedStats()

			// Get route ID from context
			routeID := m.getRouteID(r)

			// Try to match mock rules
			result := m.matchMockRules(r, routeID)
			if result.Matched {
				// Serve mock response
				m.serveMockResponse(w, r, result)
				return
			}

			// No mock matched, continue to next handler
			next.ServeHTTP(w, r)
		})
	}
}

// compileRules compiles all mock rules and sorts them by priority
func (m *MockResponseMiddleware) compileRules() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.compileRulesUnsafe()
}

// compileRulesUnsafe compiles rules without acquiring locks (assumes caller holds lock)
func (m *MockResponseMiddleware) compileRulesUnsafe() error {
	m.compiledRules = make([]*compiledMockRule, 0, len(m.config.Rules))

	for _, rule := range m.config.Rules {
		if !rule.Enabled {
			continue
		}

		compiledRule := &compiledMockRule{
			rule:        &rule,
			pathRegexes: make([]*regexp.Regexp, 0, len(rule.Conditions.Paths)),
		}

		// Compile path patterns
		for _, pathMatcher := range rule.Conditions.Paths {
			if pathMatcher.Type == "regex" {
				regex, err := regexp.Compile(pathMatcher.Value)
				if err != nil {
					return err
				}
				compiledRule.pathRegexes = append(compiledRule.pathRegexes, regex)
			}
		}

		m.compiledRules = append(m.compiledRules, compiledRule)
	}

	// Sort rules by priority (higher priority first)
	sort.Slice(m.compiledRules, func(i, j int) bool {
		return m.compiledRules[i].rule.Priority > m.compiledRules[j].rule.Priority
	})

	return nil
}

// matchMockRules tries to match request against mock rules
func (m *MockResponseMiddleware) matchMockRules(r *http.Request, routeID string) *MockMatchResult {
	m.mu.RLock()

	// Check per-route rules first
	if routeConfig, exists := m.config.PerRoute[routeID]; exists && routeConfig.Enabled {
		for _, rule := range routeConfig.Rules {
			if rule.Enabled && m.matchRule(r, &rule) {
				m.mu.RUnlock()
				m.updateMatchedStats(rule.ID)
				return &MockMatchResult{
					Matched:   true,
					Rule:      &rule,
					RuleID:    rule.ID,
					MatchedAt: time.Now(),
				}
			}
		}
	}

	// Check global rules
	for _, compiledRule := range m.compiledRules {
		if m.matchCompiledRule(r, compiledRule) {
			ruleID := compiledRule.rule.ID
			rule := compiledRule.rule
			m.mu.RUnlock()
			m.updateMatchedStats(ruleID)
			return &MockMatchResult{
				Matched:   true,
				Rule:      rule,
				RuleID:    ruleID,
				MatchedAt: time.Now(),
			}
		}
	}

	m.mu.RUnlock()
	return &MockMatchResult{Matched: false}
}

// matchRule checks if a request matches a mock rule
func (m *MockResponseMiddleware) matchRule(r *http.Request, rule *config.MockRule) bool {
	// Check HTTP methods
	if len(rule.Conditions.Methods) > 0 {
		methodMatched := false
		for _, method := range rule.Conditions.Methods {
			if strings.EqualFold(r.Method, method) {
				methodMatched = true
				break
			}
		}
		if !methodMatched {
			return false
		}
	}

	// Check paths
	if len(rule.Conditions.Paths) > 0 {
		pathMatched := false
		for _, pathMatcher := range rule.Conditions.Paths {
			if m.matchPath(r.URL.Path, pathMatcher) {
				pathMatched = true
				break
			}
		}
		if !pathMatched {
			return false
		}
	}

	// Check headers
	for headerName, expectedValue := range rule.Conditions.Headers {
		actualValue := r.Header.Get(headerName)
		if actualValue != expectedValue {
			return false
		}
	}

	// Check query parameters
	for paramName, expectedValue := range rule.Conditions.QueryParams {
		actualValue := r.URL.Query().Get(paramName)
		if actualValue != expectedValue {
			return false
		}
	}

	// Check request body if specified
	if rule.Conditions.Body != "" {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return false
		}
		// Restore body for potential future use
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		
		if string(body) != rule.Conditions.Body {
			return false
		}
	}

	return true
}

// matchCompiledRule checks if a request matches a compiled mock rule
func (m *MockResponseMiddleware) matchCompiledRule(r *http.Request, compiledRule *compiledMockRule) bool {
	rule := compiledRule.rule

	// Check HTTP methods
	if len(rule.Conditions.Methods) > 0 {
		methodMatched := false
		for _, method := range rule.Conditions.Methods {
			if strings.EqualFold(r.Method, method) {
				methodMatched = true
				break
			}
		}
		if !methodMatched {
			return false
		}
	}

	// Check paths with compiled regex
	if len(rule.Conditions.Paths) > 0 {
		pathMatched := false
		regexIndex := 0
		for _, pathMatcher := range rule.Conditions.Paths {
			if pathMatcher.Type == "regex" {
				if regexIndex < len(compiledRule.pathRegexes) {
					if compiledRule.pathRegexes[regexIndex].MatchString(r.URL.Path) {
						pathMatched = true
						break
					}
					regexIndex++
				}
			} else if m.matchPath(r.URL.Path, pathMatcher) {
				pathMatched = true
				break
			}
		}
		if !pathMatched {
			return false
		}
	}

	// Check headers
	for headerName, expectedValue := range rule.Conditions.Headers {
		actualValue := r.Header.Get(headerName)
		if actualValue != expectedValue {
			return false
		}
	}

	// Check query parameters
	for paramName, expectedValue := range rule.Conditions.QueryParams {
		actualValue := r.URL.Query().Get(paramName)
		if actualValue != expectedValue {
			return false
		}
	}

	return true
}

// matchPath checks if a path matches a path matcher
func (m *MockResponseMiddleware) matchPath(requestPath string, pathMatcher config.MockPathMatcher) bool {
	switch pathMatcher.Type {
	case "exact":
		return requestPath == pathMatcher.Value
	case "prefix":
		return strings.HasPrefix(requestPath, pathMatcher.Value)
	case "regex":
		// For non-compiled regex (fallback)
		regex, err := regexp.Compile(pathMatcher.Value)
		if err != nil {
			return false
		}
		return regex.MatchString(requestPath)
	default:
		return false
	}
}

// serveMockResponse serves a mock response
func (m *MockResponseMiddleware) serveMockResponse(w http.ResponseWriter, r *http.Request, result *MockMatchResult) {
	rule := result.Rule

	// Apply delay if configured
	if rule.Response.Delay > 0 {
		time.Sleep(rule.Response.Delay)
	}

	// Set response headers
	for key, value := range rule.Response.Headers {
		w.Header().Set(key, value)
	}

	// Set status code
	statusCode := rule.Response.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	w.WriteHeader(statusCode)

	// Write response body
	var responseBody string
	if rule.Response.BodyFile != "" {
		// Read from file
		if content, err := os.ReadFile(rule.Response.BodyFile); err == nil {
			responseBody = string(content)
		} else {
			responseBody = `{"error": "Failed to read response file"}`
		}
	} else {
		responseBody = rule.Response.Body
	}

	// Expand dynamic values in response body
	responseBody = m.expandResponseBody(responseBody, r)

	w.Write([]byte(responseBody))

	// Update statistics
	m.updateServedStats(rule.ID)
}

// expandResponseBody expands dynamic values in response body
func (m *MockResponseMiddleware) expandResponseBody(body string, r *http.Request) string {
	// Replace common placeholders
	expanded := body
	expanded = strings.ReplaceAll(expanded, "${timestamp}", time.Now().Format(time.RFC3339))
	expanded = strings.ReplaceAll(expanded, "${method}", r.Method)
	expanded = strings.ReplaceAll(expanded, "${path}", r.URL.Path)
	expanded = strings.ReplaceAll(expanded, "${host}", r.Host)
	expanded = strings.ReplaceAll(expanded, "${query}", r.URL.RawQuery)

	// Replace request header values
	if strings.Contains(expanded, "${header:") {
		// Extract header name from ${header:X-Header-Name}
		start := strings.Index(expanded, "${header:")
		for start != -1 {
			end := strings.Index(expanded[start:], "}")
			if end != -1 {
				headerName := expanded[start+9 : start+end]
				headerValue := r.Header.Get(headerName)
				placeholder := expanded[start : start+end+1]
				expanded = strings.ReplaceAll(expanded, placeholder, headerValue)
			}
			// Find next occurrence after current position
			nextStart := strings.Index(expanded[start+1:], "${header:")
			if nextStart != -1 {
				start = start + 1 + nextStart
			} else {
				start = -1
			}
		}
	}

	// Replace query parameter values
	if strings.Contains(expanded, "${query:") {
		start := strings.Index(expanded, "${query:")
		for start != -1 {
			end := strings.Index(expanded[start:], "}")
			if end != -1 {
				paramName := expanded[start+8 : start+end]
				paramValue := r.URL.Query().Get(paramName)
				placeholder := expanded[start : start+end+1]
				expanded = strings.ReplaceAll(expanded, placeholder, paramValue)
			}
			// Find next occurrence after current position
			nextStart := strings.Index(expanded[start+1:], "${query:")
			if nextStart != -1 {
				start = start + 1 + nextStart
			} else {
				start = -1
			}
		}
	}

	return expanded
}

// getRouteID extracts route ID from request context
func (m *MockResponseMiddleware) getRouteID(r *http.Request) string {
	if routeID := r.Context().Value("route_id"); routeID != nil {
		if id, ok := routeID.(string); ok {
			return id
		}
	}
	return "default"
}

// updateProcessedStats updates processed request statistics
func (m *MockResponseMiddleware) updateProcessedStats() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.RequestsProcessed++
	m.stats.LastProcessedAt = time.Now()
}

// updateMatchedStats updates matched rule statistics
func (m *MockResponseMiddleware) updateMatchedStats(ruleID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.MocksMatched++
	m.stats.RuleHits[ruleID]++
	m.stats.LastMatchedAt = time.Now()
}

// updateServedStats updates served mock response statistics
func (m *MockResponseMiddleware) updateServedStats(ruleID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.MocksServed++
}

// GetStats returns current statistics
func (m *MockResponseMiddleware) GetStats() *MockResponseStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to avoid race conditions
	statsCopy := *m.stats
	statsCopy.RuleHits = make(map[string]int64)
	for k, v := range m.stats.RuleHits {
		statsCopy.RuleHits[k] = v
	}
	return &statsCopy
}

// UpdateConfig updates the middleware configuration
func (m *MockResponseMiddleware) UpdateConfig(config *config.MockResponseConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = config
	return m.compileRulesUnsafe()
}

// ResetStats resets all statistics
func (m *MockResponseMiddleware) ResetStats() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats = &MockResponseStats{
		RuleHits:        make(map[string]int64),
		LastProcessedAt: time.Now(),
	}
}

// GetRules returns all compiled rules
func (m *MockResponseMiddleware) GetRules() []*config.MockRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*config.MockRule, 0, len(m.compiledRules))
	for _, compiledRule := range m.compiledRules {
		rules = append(rules, compiledRule.rule)
	}
	return rules
}
