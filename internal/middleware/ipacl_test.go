package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/songzhibin97/stargate/internal/config"
)

func TestIPACLMiddleware_Handler(t *testing.T) {
	// Create test handler that just returns OK
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	tests := []struct {
		name           string
		config         *config.IPACLConfig
		clientIP       string
		expectedStatus int
		expectedBody   string
		checkHeaders   func(t *testing.T, r *http.Request)
	}{
		{
			name: "Disabled middleware allows all",
			config: &config.IPACLConfig{
				Enabled: false,
			},
			clientIP:       "192.168.1.100",
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
		{
			name: "No rules allows all",
			config: &config.IPACLConfig{
				Enabled: true,
			},
			clientIP:       "192.168.1.100",
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
		{
			name: "IP in whitelist is allowed",
			config: &config.IPACLConfig{
				Enabled:   true,
				Whitelist: []string{"192.168.1.0/24", "10.0.0.1"},
			},
			clientIP:       "192.168.1.100",
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
			checkHeaders: func(t *testing.T, r *http.Request) {
				if r.Header.Get("X-Client-IP") != "192.168.1.100" {
					t.Error("Expected X-Client-IP header to be set")
				}
				if r.Header.Get("X-IP-Whitelisted") != "true" {
					t.Error("Expected X-IP-Whitelisted header to be true")
				}
			},
		},
		{
			name: "Single IP in whitelist is allowed",
			config: &config.IPACLConfig{
				Enabled:   true,
				Whitelist: []string{"10.0.0.1"},
			},
			clientIP:       "10.0.0.1",
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
		{
			name: "IP not in whitelist is blocked",
			config: &config.IPACLConfig{
				Enabled:   true,
				Whitelist: []string{"192.168.1.0/24"},
			},
			clientIP:       "10.0.0.1",
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "IP in blacklist is blocked",
			config: &config.IPACLConfig{
				Enabled:   true,
				Blacklist: []string{"192.168.1.0/24", "10.0.0.1"},
			},
			clientIP:       "192.168.1.100",
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "Single IP in blacklist is blocked",
			config: &config.IPACLConfig{
				Enabled:   true,
				Blacklist: []string{"10.0.0.1"},
			},
			clientIP:       "10.0.0.1",
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "IP not in blacklist is allowed",
			config: &config.IPACLConfig{
				Enabled:   true,
				Blacklist: []string{"192.168.1.0/24"},
			},
			clientIP:       "10.0.0.1",
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
		{
			name: "Whitelist overrides blacklist",
			config: &config.IPACLConfig{
				Enabled:   true,
				Whitelist: []string{"192.168.1.100"},
				Blacklist: []string{"192.168.1.0/24"},
			},
			clientIP:       "192.168.1.100",
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
		{
			name: "IPv6 address in whitelist",
			config: &config.IPACLConfig{
				Enabled:   true,
				Whitelist: []string{"2001:db8::/32"},
			},
			clientIP:       "2001:db8::1",
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
		{
			name: "IPv6 address in blacklist",
			config: &config.IPACLConfig{
				Enabled:   true,
				Blacklist: []string{"2001:db8::/32"},
			},
			clientIP:       "2001:db8::1",
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create middleware
			middleware, err := NewIPACLMiddleware(tt.config)
			if err != nil {
				t.Fatalf("Failed to create middleware: %v", err)
			}

			// Create handler with middleware
			handler := middleware.Handler()(testHandler)

			// Create request
			req := httptest.NewRequest("GET", "/test", nil)
			// Handle IPv6 addresses properly
			if strings.Contains(tt.clientIP, ":") && !strings.Contains(tt.clientIP, "[") {
				// IPv6 address needs brackets when combined with port
				req.RemoteAddr = "[" + tt.clientIP + "]:12345"
			} else {
				req.RemoteAddr = tt.clientIP + ":12345"
			}

			// Create response recorder
			rr := httptest.NewRecorder()

			// Serve request
			handler.ServeHTTP(rr, req)

			// Check status code
			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			// Check body for successful requests
			if tt.expectedStatus == http.StatusOK && tt.expectedBody != "" {
				if rr.Body.String() != tt.expectedBody {
					t.Errorf("Expected body %q, got %q", tt.expectedBody, rr.Body.String())
				}
			}

			// Check error response for blocked requests
			if tt.expectedStatus == http.StatusForbidden {
				var errorResp map[string]interface{}
				if err := json.Unmarshal(rr.Body.Bytes(), &errorResp); err != nil {
					t.Errorf("Failed to parse error response: %v", err)
				}

				if errorResp["error"] == nil {
					t.Error("Expected error field in response")
				}
			}

			// Check custom headers
			if tt.checkHeaders != nil {
				tt.checkHeaders(t, req)
			}
		})
	}
}

func TestIPACLMiddleware_GetClientIP(t *testing.T) {
	middleware, err := NewIPACLMiddleware(&config.IPACLConfig{Enabled: true})
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	tests := []struct {
		name       string
		setupReq   func() *http.Request
		expectedIP string
	}{
		{
			name: "X-Forwarded-For header",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Forwarded-For", "192.168.1.100, 10.0.0.1")
				req.RemoteAddr = "127.0.0.1:12345"
				return req
			},
			expectedIP: "192.168.1.100",
		},
		{
			name: "X-Real-IP header",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Real-IP", "192.168.1.100")
				req.RemoteAddr = "127.0.0.1:12345"
				return req
			},
			expectedIP: "192.168.1.100",
		},
		{
			name: "CF-Connecting-IP header",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("CF-Connecting-IP", "192.168.1.100")
				req.RemoteAddr = "127.0.0.1:12345"
				return req
			},
			expectedIP: "192.168.1.100",
		},
		{
			name: "X-Client-IP header",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Client-IP", "192.168.1.100")
				req.RemoteAddr = "127.0.0.1:12345"
				return req
			},
			expectedIP: "192.168.1.100",
		},
		{
			name: "RemoteAddr fallback",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.RemoteAddr = "192.168.1.100:12345"
				return req
			},
			expectedIP: "192.168.1.100",
		},
		{
			name: "RemoteAddr without port",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.RemoteAddr = "192.168.1.100"
				return req
			},
			expectedIP: "192.168.1.100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupReq()
			ip := middleware.getClientIP(req)

			if ip != tt.expectedIP {
				t.Errorf("Expected IP %q, got %q", tt.expectedIP, ip)
			}
		})
	}
}

func TestIPACLMiddleware_CheckIPAccess(t *testing.T) {
	tests := []struct {
		name           string
		config         *config.IPACLConfig
		clientIP       string
		expectedResult *IPACLResult
	}{
		{
			name: "No rules - allow by default",
			config: &config.IPACLConfig{
				Enabled: true,
			},
			clientIP: "192.168.1.100",
			expectedResult: &IPACLResult{
				Allowed:       true,
				Reason:        "No restrictions",
				ClientIP:      "192.168.1.100",
				IsWhitelisted: false,
				IsBlacklisted: false,
			},
		},
		{
			name: "Whitelist match",
			config: &config.IPACLConfig{
				Enabled:   true,
				Whitelist: []string{"192.168.1.0/24"},
			},
			clientIP: "192.168.1.100",
			expectedResult: &IPACLResult{
				Allowed:       true,
				Reason:        "IP is whitelisted",
				ClientIP:      "192.168.1.100",
				IsWhitelisted: true,
				IsBlacklisted: false,
				MatchedRule:   "192.168.1.0/24",
			},
		},
		{
			name: "Blacklist match",
			config: &config.IPACLConfig{
				Enabled:   true,
				Blacklist: []string{"192.168.1.0/24"},
			},
			clientIP: "192.168.1.100",
			expectedResult: &IPACLResult{
				Allowed:       false,
				Reason:        "IP is blacklisted",
				ClientIP:      "192.168.1.100",
				IsWhitelisted: false,
				IsBlacklisted: true,
				MatchedRule:   "192.168.1.0/24",
			},
		},
		{
			name: "Not in whitelist",
			config: &config.IPACLConfig{
				Enabled:   true,
				Whitelist: []string{"10.0.0.0/8"},
			},
			clientIP: "192.168.1.100",
			expectedResult: &IPACLResult{
				Allowed:       false,
				Reason:        "IP not in whitelist",
				ClientIP:      "192.168.1.100",
				IsWhitelisted: false,
				IsBlacklisted: false,
			},
		},
		{
			name: "Invalid IP",
			config: &config.IPACLConfig{
				Enabled: true,
			},
			clientIP: "invalid-ip",
			expectedResult: &IPACLResult{
				Allowed:       false,
				Reason:        "Invalid IP address",
				ClientIP:      "invalid-ip",
				IsWhitelisted: false,
				IsBlacklisted: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware, err := NewIPACLMiddleware(tt.config)
			if err != nil {
				t.Fatalf("Failed to create middleware: %v", err)
			}

			result := middleware.checkIPAccess(tt.clientIP)

			if result.Allowed != tt.expectedResult.Allowed {
				t.Errorf("Expected Allowed=%v, got %v", tt.expectedResult.Allowed, result.Allowed)
			}
			if result.Reason != tt.expectedResult.Reason {
				t.Errorf("Expected Reason=%q, got %q", tt.expectedResult.Reason, result.Reason)
			}
			if result.ClientIP != tt.expectedResult.ClientIP {
				t.Errorf("Expected ClientIP=%q, got %q", tt.expectedResult.ClientIP, result.ClientIP)
			}
			if result.IsWhitelisted != tt.expectedResult.IsWhitelisted {
				t.Errorf("Expected IsWhitelisted=%v, got %v", tt.expectedResult.IsWhitelisted, result.IsWhitelisted)
			}
			if result.IsBlacklisted != tt.expectedResult.IsBlacklisted {
				t.Errorf("Expected IsBlacklisted=%v, got %v", tt.expectedResult.IsBlacklisted, result.IsBlacklisted)
			}
			if tt.expectedResult.MatchedRule != "" && result.MatchedRule != tt.expectedResult.MatchedRule {
				t.Errorf("Expected MatchedRule=%q, got %q", tt.expectedResult.MatchedRule, result.MatchedRule)
			}
		})
	}
}

func TestIPACLMiddleware_Stats(t *testing.T) {
	config := &config.IPACLConfig{
		Enabled:   true,
		Whitelist: []string{"192.168.1.0/24"},
		Blacklist: []string{"10.0.0.0/8"},
	}

	middleware, err := NewIPACLMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Handler()(testHandler)

	// Test allowed request (whitelist)
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.168.1.100:12345"
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	// Test blocked request (blacklist)
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "10.0.0.1:12345"
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	// Check stats
	stats := middleware.GetStats()

	if stats.TotalRequests != 2 {
		t.Errorf("Expected TotalRequests=2, got %d", stats.TotalRequests)
	}
	if stats.AllowedRequests != 1 {
		t.Errorf("Expected AllowedRequests=1, got %d", stats.AllowedRequests)
	}
	if stats.BlockedRequests != 1 {
		t.Errorf("Expected BlockedRequests=1, got %d", stats.BlockedRequests)
	}
	if stats.WhitelistHits != 1 {
		t.Errorf("Expected WhitelistHits=1, got %d", stats.WhitelistHits)
	}
	if stats.BlacklistHits != 1 {
		t.Errorf("Expected BlacklistHits=1, got %d", stats.BlacklistHits)
	}
	if stats.LastBlockedIP != "10.0.0.1" {
		t.Errorf("Expected LastBlockedIP='10.0.0.1', got %q", stats.LastBlockedIP)
	}
	if stats.LastBlockedTime == nil {
		t.Error("Expected LastBlockedTime to be set")
	}

	// Test reset stats
	middleware.ResetStats()
	stats = middleware.GetStats()

	if stats.TotalRequests != 0 {
		t.Errorf("Expected TotalRequests=0 after reset, got %d", stats.TotalRequests)
	}
}
