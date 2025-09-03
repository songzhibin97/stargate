package proxy

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"golang.org/x/net/http2"
	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/tls"
	"github.com/songzhibin97/stargate/internal/tracing"
)

// Server represents the proxy server
type Server struct {
	config         *config.Config
	httpServer     *http.Server
	pipeline       *Pipeline
	acmeManager    *tls.ACMEManager
	tracerProvider *tracing.TracerProvider
}

// NewServer creates a new proxy server
func NewServer(cfg *config.Config) (*Server, error) {
	// Initialize OpenTelemetry tracing
	tracerProvider, err := tracing.NewTracerProvider(&cfg.Tracing)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tracing: %w", err)
	}

	// Create request processing pipeline
	pipeline, err := NewPipeline(cfg, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create pipeline: %w", err)
	}

	// Create ACME manager if enabled
	var acmeManager *tls.ACMEManager
	if cfg.Server.TLS.Enabled && cfg.Server.TLS.ACME.Enabled {
		acmeManager, err = tls.NewACMEManager(&cfg.Server.TLS.ACME)
		if err != nil {
			return nil, fmt.Errorf("failed to create ACME manager: %w", err)
		}
	}

	// Create HTTP server
	httpServer := &http.Server{
		Addr:           cfg.Server.Address,
		Handler:        pipeline,
		ReadTimeout:    cfg.Server.ReadTimeout,
		WriteTimeout:   cfg.Server.WriteTimeout,
		IdleTimeout:    cfg.Server.IdleTimeout,
		MaxHeaderBytes: cfg.Server.MaxHeaderBytes,
	}

	// Configure TLS if enabled
	if cfg.Server.TLS.Enabled {
		if acmeManager != nil {
			// Use ACME-managed certificates
			httpServer.TLSConfig = acmeManager.GetTLSConfig()
			// Wrap handler to handle ACME challenges
			httpServer.Handler = acmeManager.GetHTTPHandler(pipeline)
		}

		// Configure HTTP/2 support for TLS connections
		if err := http2.ConfigureServer(httpServer, &http2.Server{}); err != nil {
			log.Printf("Failed to configure HTTP/2 for proxy server: %v", err)
		} else {
			log.Println("HTTP/2 support enabled for proxy server TLS connections")
		}
	}

	return &Server{
		config:         cfg,
		httpServer:     httpServer,
		pipeline:       pipeline,
		acmeManager:    acmeManager,
		tracerProvider: tracerProvider,
	}, nil
}

// Start starts the proxy server
func (s *Server) Start() error {
	// Start the pipeline
	if err := s.pipeline.Start(); err != nil {
		return fmt.Errorf("failed to start pipeline: %w", err)
	}

	// Start ACME manager if enabled
	if s.acmeManager != nil {
		if err := s.acmeManager.Start(); err != nil {
			return fmt.Errorf("failed to start ACME manager: %w", err)
		}
		log.Printf("ACME manager started for domains: %v", s.acmeManager.GetDomains())
	}

	// Start HTTP server
	if s.config.Server.TLS.Enabled {
		if s.acmeManager != nil {
			// Use ACME-managed certificates
			return s.httpServer.ListenAndServeTLS("", "")
		} else {
			// Use static certificates
			return s.httpServer.ListenAndServeTLS(
				s.config.Server.TLS.CertFile,
				s.config.Server.TLS.KeyFile,
			)
		}
	}

	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	// Stop ACME manager first
	if s.acmeManager != nil {
		if err := s.acmeManager.Stop(); err != nil {
			log.Printf("Failed to stop ACME manager: %v", err)
		}
	}

	// Stop the pipeline
	if err := s.pipeline.Stop(); err != nil {
		return fmt.Errorf("failed to stop pipeline: %w", err)
	}

	// Shutdown tracing
	if s.tracerProvider != nil {
		if err := tracing.ShutdownGlobalTracer(ctx, s.tracerProvider); err != nil {
			log.Printf("Failed to shutdown tracer: %v", err)
		}
	}

	// Shutdown HTTP server
	return s.httpServer.Shutdown(ctx)
}

// Health returns the health status of the server
func (s *Server) Health() map[string]interface{} {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"server": map[string]interface{}{
			"address": s.config.Server.Address,
			"uptime":  time.Since(s.pipeline.startTime).Seconds(),
		},
	}

	// Add pipeline health
	if pipelineHealth := s.pipeline.Health(); pipelineHealth != nil {
		health["pipeline"] = pipelineHealth
	}

	return health
}

// Metrics returns server metrics
func (s *Server) Metrics() map[string]interface{} {
	metrics := map[string]interface{}{
		"server": map[string]interface{}{
			"address": s.config.Server.Address,
			"uptime":  time.Since(s.pipeline.startTime).Seconds(),
		},
	}

	// Add pipeline metrics
	if pipelineMetrics := s.pipeline.Metrics(); pipelineMetrics != nil {
		metrics["pipeline"] = pipelineMetrics
	}

	return metrics
}

// Reload reloads the server configuration
func (s *Server) Reload(cfg *config.Config) error {
	// Update configuration
	s.config = cfg

	// Reload pipeline
	return s.pipeline.Reload(cfg)
}
