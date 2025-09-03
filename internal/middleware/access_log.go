package middleware

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/pkg/log"
)

// AccessLogMiddleware provides structured access logging
type AccessLogMiddleware struct {
	config *config.AccessLogConfig
	writer io.Writer
	mu     sync.RWMutex
}

// AccessLogEntry represents a structured access log entry
type AccessLogEntry struct {
	Timestamp   string  `json:"timestamp"`
	ClientIP    string  `json:"client_ip"`
	Method      string  `json:"method"`
	Path        string  `json:"path"`
	StatusCode  int     `json:"status_code"`
	LatencyMs   int64   `json:"latency_ms"`
	UserAgent   string  `json:"user_agent"`
	RouteID     string  `json:"route_id,omitempty"`
	RequestSize int64   `json:"request_size"`
	ResponseSize int64  `json:"response_size"`
	Protocol    string  `json:"protocol"`
	Host        string  `json:"host"`
	Referer     string  `json:"referer,omitempty"`
	XForwardedFor string `json:"x_forwarded_for,omitempty"`
	XRealIP     string  `json:"x_real_ip,omitempty"`
}

// accessLogResponseWrapper wraps http.ResponseWriter to capture response details
type accessLogResponseWrapper struct {
	http.ResponseWriter
	statusCode   int
	responseSize int64
	wroteHeader  bool
}

func (rw *accessLogResponseWrapper) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.statusCode = code
		rw.wroteHeader = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *accessLogResponseWrapper) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.responseSize += int64(n)
	return n, err
}

// NewAccessLogMiddleware creates a new access log middleware
func NewAccessLogMiddleware(cfg *config.AccessLogConfig) (*AccessLogMiddleware, error) {
	if cfg == nil {
		return nil, fmt.Errorf("access log config cannot be nil")
	}

	var writer io.Writer
	switch cfg.Output {
	case "stdout", "":
		writer = os.Stdout
	case "stderr":
		writer = os.Stderr
	default:
		// Assume it's a file path
		file, err := os.OpenFile(cfg.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file %s: %w", cfg.Output, err)
		}
		writer = file
	}

	return &AccessLogMiddleware{
		config: cfg,
		writer: writer,
	}, nil
}

// Handler returns the HTTP middleware handler
func (m *AccessLogMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip if middleware is disabled
			if !m.config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()

			// Wrap response writer to capture response details
			wrapper := &accessLogResponseWrapper{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Process request
			next.ServeHTTP(wrapper, r)

			// Calculate latency
			latency := time.Since(start)

			// Create log entry
			entry := m.createLogEntry(r, wrapper, latency)

			// Write log entry
			m.writeLogEntry(entry)
		})
	}
}

// createLogEntry creates a structured log entry from request and response data
func (m *AccessLogMiddleware) createLogEntry(r *http.Request, wrapper *accessLogResponseWrapper, latency time.Duration) *AccessLogEntry {
	entry := &AccessLogEntry{
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		ClientIP:     m.getClientIP(r),
		Method:       r.Method,
		Path:         r.URL.Path,
		StatusCode:   wrapper.statusCode,
		LatencyMs:    latency.Nanoseconds() / 1000000, // Convert to milliseconds
		UserAgent:    r.UserAgent(),
		RequestSize:  r.ContentLength,
		ResponseSize: wrapper.responseSize,
		Protocol:     r.Proto,
		Host:         r.Host,
		Referer:      r.Referer(),
	}

	// Add query parameters to path if present
	if r.URL.RawQuery != "" {
		entry.Path = r.URL.Path + "?" + r.URL.RawQuery
	}

	// Extract route ID from context if available
	if routeID := r.Context().Value("route_id"); routeID != nil {
		if id, ok := routeID.(string); ok {
			entry.RouteID = id
		}
	}

	// Add forwarded headers
	if xForwardedFor := r.Header.Get("X-Forwarded-For"); xForwardedFor != "" {
		entry.XForwardedFor = xForwardedFor
	}
	if xRealIP := r.Header.Get("X-Real-IP"); xRealIP != "" {
		entry.XRealIP = xRealIP
	}

	return entry
}

// getClientIP extracts the real client IP from the request
func (m *AccessLogMiddleware) getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		if ips := strings.Split(xff, ","); len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if ip != "" {
				return ip
			}
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return ip
	}

	return r.RemoteAddr
}

// writeLogEntry writes the log entry to the configured output
func (m *AccessLogMiddleware) writeLogEntry(entry *AccessLogEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Use structured logging instead of direct fmt output
	logger := log.Component("middleware.access_log")

	switch m.config.Format {
	case "json", "":
		// JSON format (default) - use structured logging
		logger.Info("Access log entry",
			log.String("client_ip", entry.ClientIP),
			log.String("method", entry.Method),
			log.String("path", entry.Path),
			log.String("protocol", entry.Protocol),
			log.Int("status_code", entry.StatusCode),
			log.Int64("response_size", entry.ResponseSize),
			log.String("referer", entry.Referer),
			log.String("user_agent", entry.UserAgent),
			log.Int64("latency_ms", entry.LatencyMs),
		)
	case "combined":
		// Apache Combined Log Format
		logLine := fmt.Sprintf("%s - - [%s] \"%s %s %s\" %d %d \"%s\" \"%s\"",
			entry.ClientIP,
			time.Now().Format("02/Jan/2006:15:04:05 -0700"),
			entry.Method,
			entry.Path,
			entry.Protocol,
			entry.StatusCode,
			entry.ResponseSize,
			entry.Referer,
			entry.UserAgent,
		)
		fmt.Fprintln(m.writer, logLine)
	case "common":
		// Apache Common Log Format
		logLine := fmt.Sprintf("%s - - [%s] \"%s %s %s\" %d %d",
			entry.ClientIP,
			time.Now().Format("02/Jan/2006:15:04:05 -0700"),
			entry.Method,
			entry.Path,
			entry.Protocol,
			entry.StatusCode,
			entry.ResponseSize,
		)
		fmt.Fprintln(m.writer, logLine)
	}
}

// Close closes the middleware and any associated resources
func (m *AccessLogMiddleware) Close() error {
	if closer, ok := m.writer.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
