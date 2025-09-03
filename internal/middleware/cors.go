package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/songzhibin97/stargate/internal/config"
)

// CORSMiddleware handles Cross-Origin Resource Sharing (CORS)
type CORSMiddleware struct {
	config *config.CORSConfig
	mu     sync.RWMutex
}

// CORSResult represents the result of CORS processing
type CORSResult struct {
	Allowed       bool   `json:"allowed"`
	Origin        string `json:"origin"`
	Method        string `json:"method"`
	Headers       []string `json:"headers,omitempty"`
	IsPreflighted bool   `json:"is_preflighted"`
	Reason        string `json:"reason,omitempty"`
}

// NewCORSMiddleware creates a new CORS middleware
func NewCORSMiddleware(config *config.CORSConfig) *CORSMiddleware {
	return &CORSMiddleware{
		config: config,
	}
}

// Handler returns the HTTP middleware handler
func (c *CORSMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip if CORS is disabled
			if !c.config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Process CORS
			result := c.processCORS(w, r)

			// Handle preflight requests
			if r.Method == "OPTIONS" && result.IsPreflighted {
				if result.Allowed {
					c.handlePreflightRequest(w, r, result)
				} else {
					// CORS not allowed for preflight, return 403
					w.WriteHeader(http.StatusForbidden)
					w.Write([]byte("CORS policy violation"))
				}
				return
			}

			// Handle actual requests
			if result.Allowed {
				c.setActualRequestHeaders(w, r, result)
				next.ServeHTTP(w, r)
			} else {
				// CORS not allowed, return 403
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte("CORS policy violation"))
			}
		})
	}
}

// processCORS processes CORS for the request
func (c *CORSMiddleware) processCORS(w http.ResponseWriter, r *http.Request) *CORSResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	origin := r.Header.Get("Origin")
	result := &CORSResult{
		Origin: origin,
		Method: r.Method,
	}

	// If no origin header, it's not a CORS request
	if origin == "" {
		result.Allowed = true
		result.Reason = "Not a CORS request"
		return result
	}

	// Check if origin is allowed
	if !c.isOriginAllowed(origin) {
		result.Allowed = false
		result.Reason = "Origin not allowed"
		return result
	}

	// Check if it's a preflight request
	if r.Method == "OPTIONS" {
		requestMethod := r.Header.Get("Access-Control-Request-Method")
		requestHeaders := r.Header.Get("Access-Control-Request-Headers")

		if requestMethod != "" {
			result.IsPreflighted = true
			
			// Check if method is allowed
			if !c.isMethodAllowed(requestMethod) {
				result.Allowed = false
				result.Reason = "Method not allowed"
				return result
			}

			// Check if headers are allowed
			if requestHeaders != "" {
				headers := c.parseHeadersList(requestHeaders)
				if !c.areHeadersAllowed(headers) {
					result.Allowed = false
					result.Reason = "Headers not allowed"
					return result
				}
				result.Headers = headers
			}
		}
	} else {
		// Check if method is allowed for actual requests
		if !c.isMethodAllowed(r.Method) {
			result.Allowed = false
			result.Reason = "Method not allowed"
			return result
		}
	}

	result.Allowed = true
	return result
}

// isOriginAllowed checks if the origin is allowed
func (c *CORSMiddleware) isOriginAllowed(origin string) bool {
	// If allow all origins is enabled
	if c.config.AllowAllOrigins {
		return true
	}

	// Check against allowed origins list
	for _, allowedOrigin := range c.config.AllowedOrigins {
		if c.matchOrigin(origin, allowedOrigin) {
			return true
		}
	}

	return false
}

// matchOrigin checks if origin matches the allowed origin pattern
func (c *CORSMiddleware) matchOrigin(origin, pattern string) bool {
	// Exact match
	if origin == pattern {
		return true
	}

	// Wildcard match (e.g., *.example.com)
	if strings.HasPrefix(pattern, "*.") {
		domain := pattern[2:] // Remove "*."
		// Extract hostname from origin (remove protocol)
		hostname := origin
		if strings.Contains(origin, "://") {
			parts := strings.SplitN(origin, "://", 2)
			if len(parts) == 2 {
				hostname = parts[1]
			}
		}

		// Check if hostname matches the pattern
		if strings.HasSuffix(hostname, "."+domain) || hostname == domain {
			return true
		}
	}

	return false
}

// isMethodAllowed checks if the HTTP method is allowed
func (c *CORSMiddleware) isMethodAllowed(method string) bool {
	// If no methods specified, allow common methods
	if len(c.config.AllowedMethods) == 0 {
		commonMethods := []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "HEAD", "PATCH"}
		for _, m := range commonMethods {
			if method == m {
				return true
			}
		}
		return false
	}

	// Check against allowed methods
	for _, allowedMethod := range c.config.AllowedMethods {
		if method == allowedMethod {
			return true
		}
	}

	return false
}

// areHeadersAllowed checks if the headers are allowed
func (c *CORSMiddleware) areHeadersAllowed(headers []string) bool {
	// If no headers restrictions, allow all
	if len(c.config.AllowedHeaders) == 0 {
		return true
	}

	// Check each header
	for _, header := range headers {
		if !c.isHeaderAllowed(header) {
			return false
		}
	}

	return true
}

// isHeaderAllowed checks if a specific header is allowed
func (c *CORSMiddleware) isHeaderAllowed(header string) bool {
	header = strings.ToLower(strings.TrimSpace(header))

	// Always allow simple headers
	simpleHeaders := []string{
		"accept", "accept-language", "content-language", "content-type",
	}
	for _, simpleHeader := range simpleHeaders {
		if header == simpleHeader {
			return true
		}
	}

	// Check against allowed headers
	for _, allowedHeader := range c.config.AllowedHeaders {
		if strings.ToLower(allowedHeader) == header {
			return true
		}
	}

	return false
}

// parseHeadersList parses comma-separated headers list
func (c *CORSMiddleware) parseHeadersList(headersList string) []string {
	if headersList == "" {
		return nil
	}

	headers := strings.Split(headersList, ",")
	result := make([]string, 0, len(headers))

	for _, header := range headers {
		header = strings.TrimSpace(header)
		if header != "" {
			result = append(result, header)
		}
	}

	return result
}

// handlePreflightRequest handles OPTIONS preflight requests
func (c *CORSMiddleware) handlePreflightRequest(w http.ResponseWriter, r *http.Request, result *CORSResult) {
	// Set CORS headers for preflight
	c.setPreflightHeaders(w, r, result)

	// Set status and return
	w.WriteHeader(http.StatusNoContent)
}

// setPreflightHeaders sets headers for preflight requests
func (c *CORSMiddleware) setPreflightHeaders(w http.ResponseWriter, r *http.Request, result *CORSResult) {
	// Access-Control-Allow-Origin
	if c.config.AllowAllOrigins {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		w.Header().Set("Access-Control-Allow-Origin", result.Origin)
	}

	// Access-Control-Allow-Methods
	if len(c.config.AllowedMethods) > 0 {
		w.Header().Set("Access-Control-Allow-Methods", strings.Join(c.config.AllowedMethods, ", "))
	} else {
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, HEAD, PATCH")
	}

	// Access-Control-Allow-Headers
	requestHeaders := r.Header.Get("Access-Control-Request-Headers")
	if requestHeaders != "" && len(c.config.AllowedHeaders) > 0 {
		w.Header().Set("Access-Control-Allow-Headers", strings.Join(c.config.AllowedHeaders, ", "))
	} else if requestHeaders != "" {
		w.Header().Set("Access-Control-Allow-Headers", requestHeaders)
	}

	// Access-Control-Allow-Credentials
	if c.config.AllowCredentials && !c.config.AllowAllOrigins {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	// Access-Control-Max-Age
	if c.config.MaxAge > 0 {
		w.Header().Set("Access-Control-Max-Age", strconv.Itoa(int(c.config.MaxAge.Seconds())))
	}
}

// setActualRequestHeaders sets headers for actual requests
func (c *CORSMiddleware) setActualRequestHeaders(w http.ResponseWriter, r *http.Request, result *CORSResult) {
	// Access-Control-Allow-Origin
	if c.config.AllowAllOrigins {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		w.Header().Set("Access-Control-Allow-Origin", result.Origin)
	}

	// Access-Control-Allow-Credentials
	if c.config.AllowCredentials && !c.config.AllowAllOrigins {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	// Access-Control-Expose-Headers
	if len(c.config.ExposedHeaders) > 0 {
		w.Header().Set("Access-Control-Expose-Headers", strings.Join(c.config.ExposedHeaders, ", "))
	}
}

// UpdateConfig updates the CORS configuration
func (c *CORSMiddleware) UpdateConfig(config *config.CORSConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config = config
}

// GetConfig returns the current CORS configuration
func (c *CORSMiddleware) GetConfig() *config.CORSConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	// Return a copy to prevent external modifications
	configCopy := *c.config
	return &configCopy
}
