package proxy

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"time"
)

// ResponseWrapper wraps http.ResponseWriter to capture response details
type ResponseWrapper struct {
	http.ResponseWriter
	statusCode    int
	bytesWritten  int64
	startTime     time.Time
	headerWritten bool
}

// NewResponseWrapper creates a new response wrapper
func NewResponseWrapper(w http.ResponseWriter) *ResponseWrapper {
	return &ResponseWrapper{
		ResponseWriter: w,
		statusCode:     http.StatusOK, // Default to 200
		startTime:      time.Now(),
	}
}

// WriteHeader captures the status code
func (rw *ResponseWrapper) WriteHeader(code int) {
	if !rw.headerWritten {
		rw.statusCode = code
		rw.headerWritten = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

// Write captures the number of bytes written
func (rw *ResponseWrapper) Write(data []byte) (int, error) {
	if !rw.headerWritten {
		rw.WriteHeader(http.StatusOK)
	}
	
	n, err := rw.ResponseWriter.Write(data)
	rw.bytesWritten += int64(n)
	return n, err
}

// StatusCode returns the captured status code
func (rw *ResponseWrapper) StatusCode() int {
	return rw.statusCode
}

// BytesWritten returns the number of bytes written
func (rw *ResponseWrapper) BytesWritten() int64 {
	return rw.bytesWritten
}

// Duration returns the time elapsed since the wrapper was created
func (rw *ResponseWrapper) Duration() time.Duration {
	return time.Since(rw.startTime)
}

// StartTime returns the start time
func (rw *ResponseWrapper) StartTime() time.Time {
	return rw.startTime
}

// Hijack implements http.Hijacker interface if the underlying ResponseWriter supports it
func (rw *ResponseWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("hijacking not supported")
}

// Flush implements http.Flusher interface if the underlying ResponseWriter supports it
func (rw *ResponseWrapper) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Push implements http.Pusher interface if the underlying ResponseWriter supports it
func (rw *ResponseWrapper) Push(target string, opts *http.PushOptions) error {
	if pusher, ok := rw.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, opts)
	}
	return fmt.Errorf("push not supported")
}
