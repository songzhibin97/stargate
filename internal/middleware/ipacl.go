package middleware

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
)

// IPACLMiddleware handles IP-based access control (whitelist/blacklist)
type IPACLMiddleware struct {
	config     *config.IPACLConfig
	whitelist  []*net.IPNet
	blacklist  []*net.IPNet
	mu         sync.RWMutex
	stats      *IPACLStats
}

// IPACLStats tracks statistics for IP access control
type IPACLStats struct {
	TotalRequests    int64 `json:"total_requests"`
	AllowedRequests  int64 `json:"allowed_requests"`
	BlockedRequests  int64 `json:"blocked_requests"`
	WhitelistHits    int64 `json:"whitelist_hits"`
	BlacklistHits    int64 `json:"blacklist_hits"`
	LastBlockedIP    string `json:"last_blocked_ip,omitempty"`
	LastBlockedTime  *time.Time `json:"last_blocked_time,omitempty"`
	mu               sync.RWMutex
}

// IPACLResult represents the result of IP access control check
type IPACLResult struct {
	Allowed       bool   `json:"allowed"`
	Reason        string `json:"reason"`
	MatchedRule   string `json:"matched_rule,omitempty"`
	ClientIP      string `json:"client_ip"`
	IsWhitelisted bool   `json:"is_whitelisted"`
	IsBlacklisted bool   `json:"is_blacklisted"`
}

// NewIPACLMiddleware creates a new IP access control middleware
func NewIPACLMiddleware(config *config.IPACLConfig) (*IPACLMiddleware, error) {
	middleware := &IPACLMiddleware{
		config: config,
		stats:  &IPACLStats{},
	}
	
	// Parse whitelist CIDR blocks
	if err := middleware.parseWhitelist(); err != nil {
		return nil, fmt.Errorf("failed to parse whitelist: %w", err)
	}
	
	// Parse blacklist CIDR blocks
	if err := middleware.parseBlacklist(); err != nil {
		return nil, fmt.Errorf("failed to parse blacklist: %w", err)
	}
	
	return middleware, nil
}

// parseWhitelist parses whitelist CIDR blocks
func (m *IPACLMiddleware) parseWhitelist() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.whitelist = make([]*net.IPNet, 0, len(m.config.Whitelist))
	
	for _, cidr := range m.config.Whitelist {
		if cidr == "" {
			continue
		}
		
		// Handle single IP addresses (add /32 for IPv4, /128 for IPv6)
		if !strings.Contains(cidr, "/") {
			ip := net.ParseIP(cidr)
			if ip == nil {
				return fmt.Errorf("invalid IP address in whitelist: %s", cidr)
			}
			
			if ip.To4() != nil {
				cidr += "/32" // IPv4
			} else {
				cidr += "/128" // IPv6
			}
		}
		
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return fmt.Errorf("invalid CIDR in whitelist: %s - %w", cidr, err)
		}
		
		m.whitelist = append(m.whitelist, ipNet)
	}
	
	return nil
}

// parseBlacklist parses blacklist CIDR blocks
func (m *IPACLMiddleware) parseBlacklist() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.blacklist = make([]*net.IPNet, 0, len(m.config.Blacklist))
	
	for _, cidr := range m.config.Blacklist {
		if cidr == "" {
			continue
		}
		
		// Handle single IP addresses (add /32 for IPv4, /128 for IPv6)
		if !strings.Contains(cidr, "/") {
			ip := net.ParseIP(cidr)
			if ip == nil {
				return fmt.Errorf("invalid IP address in blacklist: %s", cidr)
			}
			
			if ip.To4() != nil {
				cidr += "/32" // IPv4
			} else {
				cidr += "/128" // IPv6
			}
		}
		
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return fmt.Errorf("invalid CIDR in blacklist: %s - %w", cidr, err)
		}
		
		m.blacklist = append(m.blacklist, ipNet)
	}
	
	return nil
}

// Handler returns the HTTP middleware handler
func (m *IPACLMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip if middleware is disabled
			if !m.config.Enabled {
				next.ServeHTTP(w, r)
				return
			}
			
			// Get client IP
			clientIP := m.getClientIP(r)
			
			// Check IP access control
			result := m.checkIPAccess(clientIP)
			
			// Update statistics
			m.updateStats(result)
			
			// Handle blocked requests
			if !result.Allowed {
				m.handleBlockedRequest(w, r, result)
				return
			}
			
			// Add IP information to request headers for upstream services
			m.addIPHeaders(r, result)
			
			// Continue to next handler
			next.ServeHTTP(w, r)
		})
	}
}

// getClientIP extracts the real client IP from the request
func (m *IPACLMiddleware) getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (most common)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		ips := strings.Split(xff, ",")
		clientIP := strings.TrimSpace(ips[0])
		if clientIP != "" {
			return clientIP
		}
	}
	
	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	
	// Check CF-Connecting-IP header (Cloudflare)
	if cfIP := r.Header.Get("CF-Connecting-IP"); cfIP != "" {
		return strings.TrimSpace(cfIP)
	}
	
	// Check X-Client-IP header
	if xci := r.Header.Get("X-Client-IP"); xci != "" {
		return strings.TrimSpace(xci)
	}
	
	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// If SplitHostPort fails, RemoteAddr might not have a port
		return r.RemoteAddr
	}
	
	return ip
}

// checkIPAccess checks if the IP is allowed based on whitelist/blacklist rules
func (m *IPACLMiddleware) checkIPAccess(clientIPStr string) *IPACLResult {
	result := &IPACLResult{
		ClientIP: clientIPStr,
		Allowed:  true,
		Reason:   "No restrictions",
	}
	
	// Parse client IP
	clientIP := net.ParseIP(clientIPStr)
	if clientIP == nil {
		result.Allowed = false
		result.Reason = "Invalid IP address"
		return result
	}
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Check whitelist first (whitelist has higher priority)
	for _, ipNet := range m.whitelist {
		if ipNet.Contains(clientIP) {
			result.Allowed = true
			result.IsWhitelisted = true
			result.Reason = "IP is whitelisted"
			result.MatchedRule = ipNet.String()
			return result
		}
	}
	
	// Check blacklist
	for _, ipNet := range m.blacklist {
		if ipNet.Contains(clientIP) {
			result.Allowed = false
			result.IsBlacklisted = true
			result.Reason = "IP is blacklisted"
			result.MatchedRule = ipNet.String()
			return result
		}
	}
	
	// If there's a whitelist configured but IP is not in it, block by default
	if len(m.whitelist) > 0 {
		result.Allowed = false
		result.Reason = "IP not in whitelist"
		return result
	}
	
	// Default allow if no rules match
	return result
}

// updateStats updates access control statistics
func (m *IPACLMiddleware) updateStats(result *IPACLResult) {
	m.stats.mu.Lock()
	defer m.stats.mu.Unlock()
	
	m.stats.TotalRequests++
	
	if result.Allowed {
		m.stats.AllowedRequests++
		if result.IsWhitelisted {
			m.stats.WhitelistHits++
		}
	} else {
		m.stats.BlockedRequests++
		if result.IsBlacklisted {
			m.stats.BlacklistHits++
		}
		
		// Update last blocked info
		m.stats.LastBlockedIP = result.ClientIP
		now := time.Now()
		m.stats.LastBlockedTime = &now
	}
}

// handleBlockedRequest handles requests that are blocked by IP ACL
func (m *IPACLMiddleware) handleBlockedRequest(w http.ResponseWriter, r *http.Request, result *IPACLResult) {
	// Log the blocked request
	log.Printf("IP ACL blocked request from %s: %s (matched rule: %s)", 
		result.ClientIP, result.Reason, result.MatchedRule)
	
	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Blocked-By", "IP-ACL")
	w.Header().Set("X-Blocked-Reason", result.Reason)
	
	// Create error response
	errorResponse := map[string]interface{}{
		"error": map[string]interface{}{
			"code":         "IP_ACCESS_DENIED",
			"message":      "Access denied based on IP address",
			"reason":       result.Reason,
			"client_ip":    result.ClientIP,
			"matched_rule": result.MatchedRule,
		},
		"timestamp": time.Now().Unix(),
		"path":      r.URL.Path,
	}
	
	// Set status code and write response
	w.WriteHeader(http.StatusForbidden)
	if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
		log.Printf("Failed to write IP ACL error response: %v", err)
	}
}

// addIPHeaders adds IP-related headers for upstream services
func (m *IPACLMiddleware) addIPHeaders(r *http.Request, result *IPACLResult) {
	r.Header.Set("X-Client-IP", result.ClientIP)
	r.Header.Set("X-IP-ACL-Result", "allowed")
	
	if result.IsWhitelisted {
		r.Header.Set("X-IP-Whitelisted", "true")
		r.Header.Set("X-IP-Whitelist-Rule", result.MatchedRule)
	}
	
	if result.MatchedRule != "" {
		r.Header.Set("X-IP-Matched-Rule", result.MatchedRule)
	}
}

// GetStats returns current statistics
func (m *IPACLMiddleware) GetStats() *IPACLStats {
	m.stats.mu.RLock()
	defer m.stats.mu.RUnlock()
	
	// Create a copy to avoid race conditions
	stats := &IPACLStats{
		TotalRequests:   m.stats.TotalRequests,
		AllowedRequests: m.stats.AllowedRequests,
		BlockedRequests: m.stats.BlockedRequests,
		WhitelistHits:   m.stats.WhitelistHits,
		BlacklistHits:   m.stats.BlacklistHits,
		LastBlockedIP:   m.stats.LastBlockedIP,
	}
	
	if m.stats.LastBlockedTime != nil {
		lastBlocked := *m.stats.LastBlockedTime
		stats.LastBlockedTime = &lastBlocked
	}
	
	return stats
}

// UpdateConfig updates the middleware configuration and reloads rules
func (m *IPACLMiddleware) UpdateConfig(config *config.IPACLConfig) error {
	m.config = config
	
	// Reparse whitelist and blacklist
	if err := m.parseWhitelist(); err != nil {
		return fmt.Errorf("failed to update whitelist: %w", err)
	}
	
	if err := m.parseBlacklist(); err != nil {
		return fmt.Errorf("failed to update blacklist: %w", err)
	}
	
	return nil
}

// ResetStats resets all statistics
func (m *IPACLMiddleware) ResetStats() {
	m.stats.mu.Lock()
	defer m.stats.mu.Unlock()
	
	m.stats.TotalRequests = 0
	m.stats.AllowedRequests = 0
	m.stats.BlockedRequests = 0
	m.stats.WhitelistHits = 0
	m.stats.BlacklistHits = 0
	m.stats.LastBlockedIP = ""
	m.stats.LastBlockedTime = nil
}
