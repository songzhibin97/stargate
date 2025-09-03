package middleware

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// GRPCWebMiddleware handles gRPC-Web protocol conversion
type GRPCWebMiddleware struct {
	config      *config.GRPCWebConfig
	connections map[string]*grpc.ClientConn
	mu          sync.RWMutex
	stats       *GRPCWebStats
}

// GRPCWebStats represents statistics for gRPC-Web proxy
type GRPCWebStats struct {
	RequestsProcessed   int64     `json:"requests_processed"`
	GRPCCallsSucceeded  int64     `json:"grpc_calls_succeeded"`
	GRPCCallsFailed     int64     `json:"grpc_calls_failed"`
	BytesReceived       int64     `json:"bytes_received"`
	BytesSent           int64     `json:"bytes_sent"`
	ServiceCalls        map[string]int64 `json:"service_calls"`
	LastProcessedAt     time.Time `json:"last_processed_at"`
}

// GRPCWebRequest represents a parsed gRPC-Web request
type GRPCWebRequest struct {
	Service     string
	Method      string
	ContentType string
	IsText      bool
	Message     []byte
	Metadata    metadata.MD
}

// GRPCWebResponse represents a gRPC-Web response
type GRPCWebResponse struct {
	StatusCode  int
	Headers     map[string]string
	Message     []byte
	Trailers    map[string]string
	GRPCStatus  codes.Code
	GRPCMessage string
}

const (
	// gRPC-Web content types
	ContentTypeGRPCWeb     = "application/grpc-web"
	ContentTypeGRPCWebProto = "application/grpc-web+proto"
	ContentTypeGRPCWebText  = "application/grpc-web-text"
	ContentTypeGRPCWebTextProto = "application/grpc-web-text+proto"
	
	// gRPC-Web headers
	HeaderGRPCWeb      = "X-Grpc-Web"
	HeaderUserAgent    = "X-User-Agent"
	HeaderGRPCStatus   = "Grpc-Status"
	HeaderGRPCMessage  = "Grpc-Message"
	HeaderGRPCTimeout  = "Grpc-Timeout"
)

// NewGRPCWebMiddleware creates a new gRPC-Web middleware
func NewGRPCWebMiddleware(config *config.GRPCWebConfig) (*GRPCWebMiddleware, error) {
	middleware := &GRPCWebMiddleware{
		config:      config,
		connections: make(map[string]*grpc.ClientConn),
		stats: &GRPCWebStats{
			ServiceCalls:    make(map[string]int64),
			LastProcessedAt: time.Now(),
		},
	}

	// Initialize gRPC connections for configured services
	if err := middleware.initializeConnections(); err != nil {
		return nil, fmt.Errorf("failed to initialize gRPC connections: %w", err)
	}

	return middleware, nil
}

// Handler returns the HTTP middleware handler
func (m *GRPCWebMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip if middleware is disabled
			if !m.config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Check if this is a gRPC-Web request
			if !m.isGRPCWebRequest(r) {
				next.ServeHTTP(w, r)
				return
			}

			// Handle CORS preflight requests
			if r.Method == http.MethodOptions {
				m.handleCORSPreflight(w, r)
				return
			}

			// Process gRPC-Web request
			m.handleGRPCWebRequest(w, r)
		})
	}
}

// isGRPCWebRequest checks if the request is a gRPC-Web request
func (m *GRPCWebMiddleware) isGRPCWebRequest(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	return strings.HasPrefix(contentType, "application/grpc-web")
}

// handleCORSPreflight handles CORS preflight requests
func (m *GRPCWebMiddleware) handleCORSPreflight(w http.ResponseWriter, r *http.Request) {
	if !m.config.CORS.Enabled {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	origin := r.Header.Get("Origin")
	if !m.isOriginAllowed(origin) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", strings.Join(m.config.CORS.AllowedMethods, ", "))
	w.Header().Set("Access-Control-Allow-Headers", strings.Join(m.config.CORS.AllowedHeaders, ", "))
	w.Header().Set("Access-Control-Max-Age", strconv.Itoa(m.config.CORS.MaxAge))
	
	if m.config.CORS.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	w.WriteHeader(http.StatusOK)
}

// handleGRPCWebRequest processes a gRPC-Web request
func (m *GRPCWebMiddleware) handleGRPCWebRequest(w http.ResponseWriter, r *http.Request) {
	// Update statistics
	m.updateProcessedStats()

	// Set CORS headers for actual requests
	if m.config.CORS.Enabled {
		origin := r.Header.Get("Origin")
		if m.isOriginAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Expose-Headers", strings.Join(m.config.CORS.ExposedHeaders, ", "))
			if m.config.CORS.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
		}
	}

	// Parse gRPC-Web request
	grpcReq, err := m.parseGRPCWebRequest(r)
	if err != nil {
		m.writeErrorResponse(w, codes.InvalidArgument, fmt.Sprintf("Failed to parse request: %v", err))
		return
	}

	// Get gRPC connection for the service
	conn, err := m.getConnection(grpcReq.Service)
	if err != nil {
		m.writeErrorResponse(w, codes.Unavailable, fmt.Sprintf("Service unavailable: %v", err))
		return
	}

	// Make gRPC call
	response, err := m.makeGRPCCall(conn, grpcReq)
	if err != nil {
		m.updateFailedStats(grpcReq.Service)
		m.writeErrorResponse(w, codes.Internal, fmt.Sprintf("gRPC call failed: %v", err))
		return
	}

	// Write gRPC-Web response
	m.writeGRPCWebResponse(w, response, grpcReq.IsText)
	m.updateSuccessStats(grpcReq.Service)
}

// parseGRPCWebRequest parses an HTTP request into a gRPC-Web request
func (m *GRPCWebMiddleware) parseGRPCWebRequest(r *http.Request) (*GRPCWebRequest, error) {
	// Extract service and method from URL path
	// Expected format: /{service}/{method}
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) != 2 {
		return nil, fmt.Errorf("invalid path format, expected /{service}/{method}")
	}

	service := pathParts[0]
	method := pathParts[1]

	// Get content type
	contentType := r.Header.Get("Content-Type")
	isText := strings.Contains(contentType, "text")

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	// Decode message
	var message []byte
	if isText {
		// Decode base64 for text format
		decoded, err := base64.StdEncoding.DecodeString(string(body))
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 message: %w", err)
		}
		message = decoded
	} else {
		message = body
	}

	// Parse gRPC message (skip 5-byte prefix if present)
	if len(message) >= 5 {
		// Check if this looks like a gRPC message with prefix
		if message[0] == 0 { // compression flag
			messageLen := binary.BigEndian.Uint32(message[1:5])
			if int(messageLen) == len(message)-5 {
				message = message[5:] // Skip the 5-byte prefix
			}
		}
	}

	// Extract metadata from headers
	md := metadata.New(nil)
	for key, values := range r.Header {
		if strings.HasPrefix(strings.ToLower(key), "grpc-") || 
		   key == HeaderUserAgent || key == HeaderGRPCTimeout {
			for _, value := range values {
				md.Append(key, value)
			}
		}
	}

	return &GRPCWebRequest{
		Service:     service,
		Method:      method,
		ContentType: contentType,
		IsText:      isText,
		Message:     message,
		Metadata:    md,
	}, nil
}

// initializeConnections initializes gRPC connections for configured services
func (m *GRPCWebMiddleware) initializeConnections() error {
	for serviceName, serviceConfig := range m.config.Services {
		if !serviceConfig.Enabled {
			continue
		}

		var opts []grpc.DialOption
		
		// Configure TLS
		if serviceConfig.TLS.Enabled {
			// TODO: Add TLS configuration
			opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		} else {
			opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		}

		// Create connection
		conn, err := grpc.Dial(serviceConfig.Backend, opts...)
		if err != nil {
			return fmt.Errorf("failed to connect to service %s at %s: %w", serviceName, serviceConfig.Backend, err)
		}

		m.connections[serviceName] = conn
	}

	return nil
}

// getConnection gets a gRPC connection for a service
func (m *GRPCWebMiddleware) getConnection(service string) (*grpc.ClientConn, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conn, exists := m.connections[service]
	if !exists {
		return nil, fmt.Errorf("no connection configured for service: %s", service)
	}

	return conn, nil
}

// makeGRPCCall makes a gRPC call to the backend service
func (m *GRPCWebMiddleware) makeGRPCCall(conn *grpc.ClientConn, req *GRPCWebRequest) (*GRPCWebResponse, error) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), m.config.DefaultTimeout)
	defer cancel()

	// Add metadata to context
	ctx = metadata.NewOutgoingContext(ctx, req.Metadata)

	// Prepare method name
	fullMethod := fmt.Sprintf("/%s/%s", req.Service, req.Method)

	// Make unary call
	var response []byte
	var header, trailer metadata.MD

	err := conn.Invoke(ctx, fullMethod, req.Message, &response,
		grpc.Header(&header), grpc.Trailer(&trailer))

	grpcResp := &GRPCWebResponse{
		StatusCode: http.StatusOK,
		Headers:    make(map[string]string),
		Trailers:   make(map[string]string),
	}

	if err != nil {
		// Handle gRPC error
		if st, ok := status.FromError(err); ok {
			grpcResp.GRPCStatus = st.Code()
			grpcResp.GRPCMessage = st.Message()
			grpcResp.StatusCode = grpcStatusToHTTPStatus(st.Code())
		} else {
			grpcResp.GRPCStatus = codes.Internal
			grpcResp.GRPCMessage = err.Error()
			grpcResp.StatusCode = http.StatusInternalServerError
		}
	} else {
		grpcResp.Message = response
		grpcResp.GRPCStatus = codes.OK
	}

	// Convert metadata to headers
	for key, values := range header {
		if len(values) > 0 {
			grpcResp.Headers[key] = values[0]
		}
	}

	for key, values := range trailer {
		if len(values) > 0 {
			grpcResp.Trailers[key] = values[0]
		}
	}

	return grpcResp, nil
}

// writeGRPCWebResponse writes a gRPC-Web response
func (m *GRPCWebMiddleware) writeGRPCWebResponse(w http.ResponseWriter, resp *GRPCWebResponse, isText bool) {
	// Set content type
	if isText {
		w.Header().Set("Content-Type", ContentTypeGRPCWebText)
	} else {
		w.Header().Set("Content-Type", ContentTypeGRPCWeb)
	}

	// Set gRPC status headers
	w.Header().Set(HeaderGRPCStatus, strconv.Itoa(int(resp.GRPCStatus)))
	if resp.GRPCMessage != "" {
		w.Header().Set(HeaderGRPCMessage, resp.GRPCMessage)
	}

	// Set other headers
	for key, value := range resp.Headers {
		w.Header().Set(key, value)
	}

	// Write status code
	w.WriteHeader(resp.StatusCode)

	// Prepare response body
	var responseBody []byte

	if resp.Message != nil {
		// Add 5-byte prefix (compression flag + message length)
		prefix := make([]byte, 5)
		prefix[0] = 0 // no compression
		binary.BigEndian.PutUint32(prefix[1:], uint32(len(resp.Message)))

		responseBody = append(prefix, resp.Message...)
	}

	// Add trailers if any
	if len(resp.Trailers) > 0 {
		// gRPC-Web trailers are sent as a special message
		trailerData := m.encodeTrailers(resp.Trailers)
		trailerPrefix := make([]byte, 5)
		trailerPrefix[0] = 0x80 // trailer flag
		binary.BigEndian.PutUint32(trailerPrefix[1:], uint32(len(trailerData)))

		responseBody = append(responseBody, trailerPrefix...)
		responseBody = append(responseBody, trailerData...)
	}

	// Encode as base64 if text format
	if isText && len(responseBody) > 0 {
		encoded := base64.StdEncoding.EncodeToString(responseBody)
		responseBody = []byte(encoded)
	}

	// Write response body
	if len(responseBody) > 0 {
		w.Write(responseBody)
		m.updateBytesSent(int64(len(responseBody)))
	}
}

// writeErrorResponse writes an error response
func (m *GRPCWebMiddleware) writeErrorResponse(w http.ResponseWriter, code codes.Code, message string) {
	w.Header().Set("Content-Type", ContentTypeGRPCWeb)
	w.Header().Set(HeaderGRPCStatus, strconv.Itoa(int(code)))
	w.Header().Set(HeaderGRPCMessage, message)

	statusCode := grpcStatusToHTTPStatus(code)
	w.WriteHeader(statusCode)
}

// encodeTrailers encodes gRPC trailers for gRPC-Web
func (m *GRPCWebMiddleware) encodeTrailers(trailers map[string]string) []byte {
	var buf bytes.Buffer
	for key, value := range trailers {
		buf.WriteString(key)
		buf.WriteString(": ")
		buf.WriteString(value)
		buf.WriteString("\r\n")
	}
	return buf.Bytes()
}

// isOriginAllowed checks if an origin is allowed
func (m *GRPCWebMiddleware) isOriginAllowed(origin string) bool {
	if len(m.config.CORS.AllowedOrigins) == 0 {
		return true
	}

	for _, allowed := range m.config.CORS.AllowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}

	return false
}

// grpcStatusToHTTPStatus converts gRPC status code to HTTP status code
func grpcStatusToHTTPStatus(code codes.Code) int {
	switch code {
	case codes.OK:
		return http.StatusOK
	case codes.Canceled:
		return http.StatusRequestTimeout
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.DeadlineExceeded:
		return http.StatusRequestTimeout
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.FailedPrecondition:
		return http.StatusBadRequest
	case codes.Aborted:
		return http.StatusConflict
	case codes.OutOfRange:
		return http.StatusBadRequest
	case codes.Unimplemented:
		return http.StatusNotImplemented
	case codes.Internal:
		return http.StatusInternalServerError
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.DataLoss:
		return http.StatusInternalServerError
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}

// Statistics methods
func (m *GRPCWebMiddleware) updateProcessedStats() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.RequestsProcessed++
	m.stats.LastProcessedAt = time.Now()
}

func (m *GRPCWebMiddleware) updateSuccessStats(service string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.GRPCCallsSucceeded++
	m.stats.ServiceCalls[service]++
}

func (m *GRPCWebMiddleware) updateFailedStats(service string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.GRPCCallsFailed++
}

func (m *GRPCWebMiddleware) updateBytesSent(bytes int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.BytesSent += bytes
}

// GetStats returns current statistics
func (m *GRPCWebMiddleware) GetStats() *GRPCWebStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to avoid race conditions
	statsCopy := *m.stats
	statsCopy.ServiceCalls = make(map[string]int64)
	for k, v := range m.stats.ServiceCalls {
		statsCopy.ServiceCalls[k] = v
	}
	return &statsCopy
}

// Close closes all gRPC connections
func (m *GRPCWebMiddleware) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, conn := range m.connections {
		if err := conn.Close(); err != nil {
			return err
		}
	}

	return nil
}
