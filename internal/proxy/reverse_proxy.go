package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/types"
)

// ReverseProxy represents the reverse proxy implementation
type ReverseProxy struct {
	config    *config.Config
	transport *http.Transport
	proxy     *httputil.ReverseProxy
}

// NewReverseProxy creates a new reverse proxy
func NewReverseProxy(cfg *config.Config) (*ReverseProxy, error) {
	// Create custom transport
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   cfg.Proxy.ConnectTimeout,
			KeepAlive: cfg.Proxy.KeepAliveTimeout,
		}).DialContext,
		ResponseHeaderTimeout: cfg.Proxy.ResponseHeaderTimeout,
		MaxIdleConns:          cfg.Proxy.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.Proxy.MaxIdleConnsPerHost,
		IdleConnTimeout:       cfg.Proxy.KeepAliveTimeout,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	rp := &ReverseProxy{
		config:    cfg,
		transport: transport,
	}

	// Create httputil.ReverseProxy with custom director
	rp.proxy = &httputil.ReverseProxy{
		Director:       rp.director,
		Transport:      transport,
		ModifyResponse: rp.modifyResponse,
		ErrorHandler:   rp.errorHandler,
		BufferPool:     &bufferPool{size: cfg.Proxy.BufferSize},
	}

	return rp, nil
}

// ServeHTTP implements http.Handler interface
func (rp *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rp.proxy.ServeHTTP(w, r)
}

// director modifies the request before forwarding
func (rp *ReverseProxy) director(req *http.Request) {
	// Get target from context (set by load balancer)
	target, ok := req.Context().Value("target").(*types.Target)
	if !ok {
		// Fallback to default target (for testing)
		target = &types.Target{
			Host: "httpbin.org",
			Port: 80,
		}
	}

	// Set target URL
	req.URL.Scheme = "http"
	if target.Port == 443 {
		req.URL.Scheme = "https"
	}
	req.URL.Host = fmt.Sprintf("%s:%d", target.Host, target.Port)

	// Preserve original host header if needed
	if req.Header.Get("X-Forwarded-Host") == "" {
		req.Header.Set("X-Forwarded-Host", req.Host)
	}

	// Set forwarded headers
	if req.Header.Get("X-Forwarded-For") == "" {
		req.Header.Set("X-Forwarded-For", getClientIP(req))
	} else {
		req.Header.Set("X-Forwarded-For", req.Header.Get("X-Forwarded-For")+", "+getClientIP(req))
	}

	if req.Header.Get("X-Forwarded-Proto") == "" {
		if req.TLS != nil {
			req.Header.Set("X-Forwarded-Proto", "https")
		} else {
			req.Header.Set("X-Forwarded-Proto", "http")
		}
	}

	// Set real IP
	if req.Header.Get("X-Real-IP") == "" {
		req.Header.Set("X-Real-IP", getClientIP(req))
	}

	// Remove hop-by-hop headers
	removeHopByHopHeaders(req.Header)

	// Set User-Agent if not present
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "Stargate/1.0")
	}

	// Inject tracing context into outbound request headers
	if rp.config.Tracing.Enabled {
		propagator := otel.GetTextMapPropagator()
		propagator.Inject(req.Context(), propagation.HeaderCarrier(req.Header))
	}
}

// modifyResponse modifies the response before returning to client
func (rp *ReverseProxy) modifyResponse(resp *http.Response) error {
	// Remove hop-by-hop headers
	removeHopByHopHeaders(resp.Header)

	// Add server header
	resp.Header.Set("Server", "Stargate/1.0")

	// Add CORS headers if needed
	if rp.shouldAddCORS(resp.Request) {
		resp.Header.Set("Access-Control-Allow-Origin", "*")
		resp.Header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		resp.Header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	}

	return nil
}

// errorHandler handles proxy errors
func (rp *ReverseProxy) errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	// Log error
	// logger.Error("proxy error", "error", err, "url", r.URL.String())

	// Determine error type and status code
	status := http.StatusBadGateway
	message := "Bad Gateway"
	isTimeout := false

	if err == context.DeadlineExceeded {
		status = http.StatusGatewayTimeout
		message = "Gateway Timeout"
		isTimeout = true
	} else if _, ok := err.(net.Error); ok {
		status = http.StatusServiceUnavailable
		message = "Service Unavailable"
	}

	// Store error information in request context for passive health checking
	ctx := context.WithValue(r.Context(), "proxy_error", err)
	ctx = context.WithValue(ctx, "proxy_timeout", isTimeout)
	ctx = context.WithValue(ctx, "proxy_status", status)
	*r = *r.WithContext(ctx)

	// Write error response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(fmt.Sprintf(`{"error": "%s", "message": "%s"}`, http.StatusText(status), message)))
}

// shouldAddCORS determines if CORS headers should be added
func (rp *ReverseProxy) shouldAddCORS(r *http.Request) bool {
	// Add CORS headers for browser requests
	origin := r.Header.Get("Origin")
	return origin != ""
}

// getClientIP extracts the client IP address
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to remote address
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// removeHopByHopHeaders removes hop-by-hop headers
func removeHopByHopHeaders(h http.Header) {
	// Connection-specific headers
	h.Del("Connection")
	h.Del("Keep-Alive")
	h.Del("Proxy-Authenticate")
	h.Del("Proxy-Authorization")
	h.Del("Te")
	h.Del("Trailers")
	h.Del("Transfer-Encoding")
	h.Del("Upgrade")
}

// bufferPool implements httputil.BufferPool
type bufferPool struct {
	size int
}

func (bp *bufferPool) Get() []byte {
	return make([]byte, bp.size)
}

func (bp *bufferPool) Put(b []byte) {
	// No-op for simplicity
}

// Health returns the health status of the reverse proxy
func (rp *ReverseProxy) Health() map[string]interface{} {
	return map[string]interface{}{
		"status": "healthy",
		"transport": map[string]interface{}{
			"max_idle_conns":          rp.transport.MaxIdleConns,
			"max_idle_conns_per_host": rp.transport.MaxIdleConnsPerHost,
			"idle_conn_timeout":       rp.transport.IdleConnTimeout.String(),
		},
	}
}

// Close closes the reverse proxy and cleans up resources
func (rp *ReverseProxy) Close() error {
	if rp.transport != nil {
		rp.transport.CloseIdleConnections()
	}
	return nil
}

// SetTarget sets the target for a request
func SetTarget(r *http.Request, target *types.Target) *http.Request {
	ctx := context.WithValue(r.Context(), "target", target)
	return r.WithContext(ctx)
}

// GetTarget gets the target from a request context
func GetTarget(r *http.Request) (*types.Target, bool) {
	target, ok := r.Context().Value("target").(*types.Target)
	return target, ok
}

// CopyHeader copies headers from source to destination
func CopyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

// CloneRequest creates a shallow copy of the request
func CloneRequest(r *http.Request) *http.Request {
	r2 := new(http.Request)
	*r2 = *r
	r2.URL = cloneURL(r.URL)
	r2.Header = cloneHeader(r.Header)
	return r2
}

// cloneURL creates a copy of the URL
func cloneURL(u *url.URL) *url.URL {
	if u == nil {
		return nil
	}
	u2 := new(url.URL)
	*u2 = *u
	if u.User != nil {
		u2.User = new(url.Userinfo)
		*u2.User = *u.User
	}
	return u2
}

// cloneHeader creates a copy of the header
func cloneHeader(h http.Header) http.Header {
	h2 := make(http.Header, len(h))
	for k, vv := range h {
		vv2 := make([]string, len(vv))
		copy(vv2, vv)
		h2[k] = vv2
	}
	return h2
}

// DrainBody reads and closes the response body
func DrainBody(resp *http.Response) {
	if resp.Body != nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}
