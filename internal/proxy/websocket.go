package proxy

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/types"
)

const (
	// WebSocket magic string for key generation
	websocketMagicString = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

	// WebSocket protocol version
	websocketVersion = "13"
)



// WebSocketProxy handles WebSocket connections
type WebSocketProxy struct {
	config    *config.Config
	mu        sync.RWMutex
	activeConns map[string]*websocketConnection
}

// websocketConnection represents an active WebSocket connection
type websocketConnection struct {
	id         string
	clientConn net.Conn
	upstreamConn net.Conn
	ctx        context.Context
	cancel     context.CancelFunc
	startTime  time.Time
}

// NewWebSocketProxy creates a new WebSocket proxy
func NewWebSocketProxy(cfg *config.Config) *WebSocketProxy {
	return &WebSocketProxy{
		config:      cfg,
		activeConns: make(map[string]*websocketConnection),
	}
}

// IsWebSocketUpgrade checks if the request is a WebSocket upgrade request
func (wp *WebSocketProxy) IsWebSocketUpgrade(r *http.Request) bool {
	// Check required headers for WebSocket upgrade
	connection := strings.ToLower(r.Header.Get("Connection"))
	upgrade := strings.ToLower(r.Header.Get("Upgrade"))
	version := r.Header.Get("Sec-WebSocket-Version")
	key := r.Header.Get("Sec-WebSocket-Key")

	// Validate WebSocket upgrade requirements
	return strings.Contains(connection, "upgrade") &&
		upgrade == "websocket" &&
		version == websocketVersion &&
		key != ""
}

// HandleWebSocketUpgrade handles the WebSocket upgrade process
func (wp *WebSocketProxy) HandleWebSocketUpgrade(w http.ResponseWriter, r *http.Request) error {
	// Get target from context (set by load balancer)
	target, ok := GetTarget(r)
	if !ok {
		return fmt.Errorf("no target found in request context")
	}

	// Hijack the client connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return fmt.Errorf("response writer does not support hijacking")
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		return fmt.Errorf("failed to hijack client connection: %w", err)
	}

	// Connect to upstream WebSocket server
	upstreamConn, err := wp.connectToUpstream(target, r)
	if err != nil {
		clientConn.Close()
		return fmt.Errorf("failed to connect to upstream: %w", err)
	}

	// Send upgrade request to upstream
	if err := wp.sendUpgradeRequest(upstreamConn, r); err != nil {
		clientConn.Close()
		upstreamConn.Close()
		return fmt.Errorf("failed to send upgrade request to upstream: %w", err)
	}

	// Read upgrade response from upstream
	upgradeResp, err := wp.readUpgradeResponse(upstreamConn)
	if err != nil {
		clientConn.Close()
		upstreamConn.Close()
		return fmt.Errorf("failed to read upgrade response from upstream: %w", err)
	}

	// Validate upstream upgrade response
	if upgradeResp.StatusCode != http.StatusSwitchingProtocols {
		clientConn.Close()
		upstreamConn.Close()
		return fmt.Errorf("upstream rejected WebSocket upgrade: %d", upgradeResp.StatusCode)
	}

	// Send upgrade response to client
	if err := wp.sendUpgradeResponse(clientConn, r); err != nil {
		clientConn.Close()
		upstreamConn.Close()
		return fmt.Errorf("failed to send upgrade response to client: %w", err)
	}

	// Create connection context
	ctx, cancel := context.WithCancel(context.Background())
	
	// Create connection record
	connID := wp.generateConnectionID(r)
	conn := &websocketConnection{
		id:           connID,
		clientConn:   clientConn,
		upstreamConn: upstreamConn,
		ctx:          ctx,
		cancel:       cancel,
		startTime:    time.Now(),
	}

	// Register connection
	wp.mu.Lock()
	wp.activeConns[connID] = conn
	wp.mu.Unlock()

	// Start bidirectional data copying
	go wp.proxyData(conn)

	return nil
}

// connectToUpstream establishes connection to upstream WebSocket server
func (wp *WebSocketProxy) connectToUpstream(target *types.Target, r *http.Request) (net.Conn, error) {
	address := fmt.Sprintf("%s:%d", target.Host, target.Port)
	
	// Use configured connect timeout
	dialer := &net.Dialer{
		Timeout: wp.config.Proxy.ConnectTimeout,
	}
	
	return dialer.Dial("tcp", address)
}

// sendUpgradeRequest sends the WebSocket upgrade request to upstream
func (wp *WebSocketProxy) sendUpgradeRequest(conn net.Conn, r *http.Request) error {
	// Build upgrade request
	req := fmt.Sprintf("%s %s HTTP/1.1\r\n", r.Method, r.RequestURI)
	req += fmt.Sprintf("Host: %s\r\n", r.Host)
	req += "Connection: Upgrade\r\n"
	req += "Upgrade: websocket\r\n"
	req += fmt.Sprintf("Sec-WebSocket-Version: %s\r\n", r.Header.Get("Sec-WebSocket-Version"))
	req += fmt.Sprintf("Sec-WebSocket-Key: %s\r\n", r.Header.Get("Sec-WebSocket-Key"))
	
	// Add optional headers
	if protocol := r.Header.Get("Sec-WebSocket-Protocol"); protocol != "" {
		req += fmt.Sprintf("Sec-WebSocket-Protocol: %s\r\n", protocol)
	}
	if extensions := r.Header.Get("Sec-WebSocket-Extensions"); extensions != "" {
		req += fmt.Sprintf("Sec-WebSocket-Extensions: %s\r\n", extensions)
	}
	
	// Add forwarded headers
	req += fmt.Sprintf("X-Forwarded-For: %s\r\n", getClientIP(r))
	req += fmt.Sprintf("X-Forwarded-Proto: %s\r\n", getProto(r))
	req += fmt.Sprintf("X-Real-IP: %s\r\n", getClientIP(r))
	
	req += "\r\n"
	
	_, err := conn.Write([]byte(req))
	return err
}

// readUpgradeResponse reads the WebSocket upgrade response from upstream
func (wp *WebSocketProxy) readUpgradeResponse(conn net.Conn) (*http.Response, error) {
	reader := bufio.NewReader(conn)
	return http.ReadResponse(reader, nil)
}

// sendUpgradeResponse sends the WebSocket upgrade response to client
func (wp *WebSocketProxy) sendUpgradeResponse(conn net.Conn, r *http.Request) error {
	key := r.Header.Get("Sec-WebSocket-Key")
	acceptKey := wp.generateAcceptKey(key)
	
	response := "HTTP/1.1 101 Switching Protocols\r\n"
	response += "Connection: Upgrade\r\n"
	response += "Upgrade: websocket\r\n"
	response += fmt.Sprintf("Sec-WebSocket-Accept: %s\r\n", acceptKey)
	response += "\r\n"
	
	_, err := conn.Write([]byte(response))
	return err
}

// generateAcceptKey generates the Sec-WebSocket-Accept key
func (wp *WebSocketProxy) generateAcceptKey(key string) string {
	h := sha1.New()
	h.Write([]byte(key + websocketMagicString))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// generateConnectionID generates a unique connection ID
func (wp *WebSocketProxy) generateConnectionID(r *http.Request) string {
	return fmt.Sprintf("%s_%d", getClientIP(r), time.Now().UnixNano())
}

// proxyData handles bidirectional data copying between client and upstream
func (wp *WebSocketProxy) proxyData(conn *websocketConnection) {
	defer wp.cleanupConnection(conn)
	
	// Create wait group for both copy operations
	var wg sync.WaitGroup
	wg.Add(2)
	
	// Copy data from client to upstream
	go func() {
		defer wg.Done()
		wp.copyData(conn.clientConn, conn.upstreamConn, "client->upstream", conn.ctx)
	}()
	
	// Copy data from upstream to client
	go func() {
		defer wg.Done()
		wp.copyData(conn.upstreamConn, conn.clientConn, "upstream->client", conn.ctx)
	}()
	
	// Wait for either direction to complete or context cancellation
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// One direction completed
	case <-conn.ctx.Done():
		// Context cancelled
	}
}

// copyData copies data from source to destination
func (wp *WebSocketProxy) copyData(src, dst net.Conn, direction string, ctx context.Context) {
	buffer := make([]byte, wp.config.Proxy.BufferSize)
	
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		
		// Set read timeout
		src.SetReadDeadline(time.Now().Add(wp.config.Proxy.KeepAliveTimeout))
		
		n, err := src.Read(buffer)
		if err != nil {
			if err != io.EOF {
				// Log error but don't spam logs for normal connection closures
				if !isConnectionClosed(err) {
					fmt.Printf("WebSocket proxy read error (%s): %v\n", direction, err)
				}
			}
			return
		}
		
		if n > 0 {
			// Set write timeout
			dst.SetWriteDeadline(time.Now().Add(wp.config.Proxy.KeepAliveTimeout))
			
			_, err := dst.Write(buffer[:n])
			if err != nil {
				if !isConnectionClosed(err) {
					fmt.Printf("WebSocket proxy write error (%s): %v\n", direction, err)
				}
				return
			}
		}
	}
}

// cleanupConnection cleans up connection resources
func (wp *WebSocketProxy) cleanupConnection(conn *websocketConnection) {
	// Cancel context
	conn.cancel()
	
	// Close connections
	if conn.clientConn != nil {
		conn.clientConn.Close()
	}
	if conn.upstreamConn != nil {
		conn.upstreamConn.Close()
	}
	
	// Remove from active connections
	wp.mu.Lock()
	delete(wp.activeConns, conn.id)
	wp.mu.Unlock()
	
	// Log connection closure
	duration := time.Since(conn.startTime)
	fmt.Printf("WebSocket connection closed: %s (duration: %v)\n", conn.id, duration)
}

// isConnectionClosed checks if error indicates connection closure
func isConnectionClosed(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "use of closed network connection")
}

// getProto returns the protocol scheme
func getProto(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	return "http"
}



// GetActiveConnections returns the number of active WebSocket connections
func (wp *WebSocketProxy) GetActiveConnections() int {
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	return len(wp.activeConns)
}

// Close closes all active WebSocket connections
func (wp *WebSocketProxy) Close() error {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	
	for _, conn := range wp.activeConns {
		conn.cancel()
		if conn.clientConn != nil {
			conn.clientConn.Close()
		}
		if conn.upstreamConn != nil {
			conn.upstreamConn.Close()
		}
	}
	
	wp.activeConns = make(map[string]*websocketConnection)
	return nil
}
