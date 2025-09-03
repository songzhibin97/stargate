package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/net/http2"
	"github.com/songzhibin97/stargate/internal/config"
)

var (
	configFile = flag.String("config", "config.yaml", "Configuration file path")
	version    = flag.Bool("version", false, "Show version information")
)

const (
	// Version information
	Version   = "v1.0.0"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// BasicHTTPServer represents a basic HTTP server
type BasicHTTPServer struct {
	config     *config.Config
	httpServer *http.Server
	mux        *http.ServeMux
}

// NewBasicHTTPServer creates a new basic HTTP server
func NewBasicHTTPServer(cfg *config.Config) *BasicHTTPServer {
	mux := http.NewServeMux()

	httpServer := &http.Server{
		Addr:           cfg.Server.Address,
		Handler:        mux,
		ReadTimeout:    cfg.Server.ReadTimeout,
		WriteTimeout:   cfg.Server.WriteTimeout,
		IdleTimeout:    cfg.Server.IdleTimeout,
		MaxHeaderBytes: cfg.Server.MaxHeaderBytes,
	}

	// Configure HTTP/2 support for TLS connections
	if cfg.Server.TLS.Enabled {
		if err := http2.ConfigureServer(httpServer, &http2.Server{}); err != nil {
			log.Printf("Failed to configure HTTP/2: %v", err)
		} else {
			log.Println("HTTP/2 support enabled for TLS connections")
		}
	}

	server := &BasicHTTPServer{
		config:     cfg,
		mux:        mux,
		httpServer: httpServer,
	}

	// Setup routes
	server.setupRoutes()

	return server
}

// setupRoutes sets up HTTP routes
func (s *BasicHTTPServer) setupRoutes() {
	// Health check endpoint
	s.mux.HandleFunc("/health", s.handleHealth)
	
	// Default handler for other routes
	s.mux.HandleFunc("/", s.handleDefault)
}

// handleHealth handles health check requests
func (s *BasicHTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	// Log protocol information
	s.logProtocolInfo(r)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Determine protocol version
	protocol := "HTTP/1.1"
	if r.ProtoMajor == 2 {
		protocol = "HTTP/2"
	}

	response := fmt.Sprintf(`{
	"status": "healthy",
	"timestamp": %d,
	"protocol": "%s",
	"server": {
		"address": "%s",
		"version": "%s"
	}
}`, time.Now().Unix(), protocol, s.config.Server.Address, Version)

	w.Write([]byte(response))
}

// logProtocolInfo logs detailed protocol information for debugging
func (s *BasicHTTPServer) logProtocolInfo(r *http.Request) {
	protocol := "HTTP/1.1"
	if r.ProtoMajor == 2 {
		protocol = "HTTP/2"
	}

	log.Printf("Request: %s %s %s - Protocol: %s, TLS: %v, Remote: %s",
		r.Method, r.URL.Path, r.Proto, protocol, r.TLS != nil, r.RemoteAddr)

	// Log additional HTTP/2 specific information
	if r.ProtoMajor == 2 {
		log.Printf("HTTP/2 Connection - Stream ID available, Multiplexing enabled")
		if r.TLS != nil {
			log.Printf("HTTP/2 over TLS - ALPN negotiated successfully")
		}
	}
}

// handleDefault handles default requests
func (s *BasicHTTPServer) handleDefault(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	
	response := fmt.Sprintf("Basic HTTP Server %s\nPath: %s\nMethod: %s\n", 
		Version, r.URL.Path, r.Method)
	
	w.Write([]byte(response))
}

// Start starts the HTTP server
func (s *BasicHTTPServer) Start() error {
	log.Printf("Starting Basic HTTP Server on %s", s.config.Server.Address)
	
	// Start HTTP server
	if s.config.Server.TLS.Enabled {
		return s.httpServer.ListenAndServeTLS(
			s.config.Server.TLS.CertFile,
			s.config.Server.TLS.KeyFile,
		)
	}
	
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *BasicHTTPServer) Shutdown(ctx context.Context) error {
	log.Println("Shutting down Basic HTTP Server...")
	return s.httpServer.Shutdown(ctx)
}

func main() {
	flag.Parse()

	if *version {
		fmt.Printf("Basic HTTP Server %s\n", Version)
		fmt.Printf("Build Time: %s\n", BuildTime)
		fmt.Printf("Git Commit: %s\n", GitCommit)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create server
	server := NewBasicHTTPServer(cfg)

	// Create a channel to receive OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-sigChan
	log.Println("Received interrupt signal, initiating graceful shutdown...")

	// Create a context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown server gracefully
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
		os.Exit(1)
	}

	log.Println("Server shutdown completed successfully")
}
