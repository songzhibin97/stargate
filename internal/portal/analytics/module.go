package analytics

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/songzhibin97/stargate/pkg/log"
	"github.com/songzhibin97/stargate/pkg/portal"
)

// Module represents the analytics module
type Module struct {
	service    *Service
	handler    *Handler
	middleware *AnalyticsMiddleware
	config     *Config
	logger     log.Logger
}

// ModuleConfig represents the configuration for the analytics module
type ModuleConfig struct {
	Analytics *Config `yaml:"analytics" json:"analytics"`
}

// NewModule creates a new analytics module
func NewModule(config *Config, appRepo portal.ApplicationRepository, logger log.Logger) (*Module, error) {
	if config == nil {
		return nil, fmt.Errorf("analytics config is required")
	}

	if config.PrometheusURL == "" {
		return nil, fmt.Errorf("prometheus_url is required in analytics config")
	}

	// Create service
	service := NewService(config, appRepo)

	// Create handler
	handler := NewHandler(service, logger)

	// Create middleware
	middleware := NewAnalyticsMiddleware(logger)

	return &Module{
		service:    service,
		handler:    handler,
		middleware: middleware,
		config:     config,
		logger:     logger,
	}, nil
}

// RegisterRoutes registers the analytics routes
func (m *Module) RegisterRoutes(router *gin.RouterGroup) {
	// Apply middleware
	analyticsGroup := router.Group("")
	analyticsGroup.Use(m.middleware.ValidationMiddleware())
	analyticsGroup.Use(m.middleware.RateLimitMiddleware())
	analyticsGroup.Use(m.middleware.CacheMiddleware())

	// Register routes
	m.handler.RegisterRoutes(analyticsGroup)

	m.logger.Info("Analytics routes registered successfully")
}

// Start starts the analytics module
func (m *Module) Start(ctx context.Context) error {
	m.logger.Info("Starting analytics module",
		log.String("prometheus_url", m.config.PrometheusURL),
		log.String("namespace", m.config.Namespace),
		log.String("subsystem", m.config.Subsystem))

	// Health check Prometheus connection
	if err := m.service.Health(ctx); err != nil {
		m.logger.Warn("Prometheus health check failed during startup", log.Error(err))
		// Don't fail startup, just log the warning
	} else {
		m.logger.Info("Prometheus connection verified successfully")
	}

	return nil
}

// Stop stops the analytics module
func (m *Module) Stop(ctx context.Context) error {
	m.logger.Info("Stopping analytics module")
	return nil
}

// Health returns the health status of the analytics module
func (m *Module) Health(ctx context.Context) error {
	return m.service.Health(ctx)
}

// GetService returns the analytics service
func (m *Module) GetService() *Service {
	return m.service
}

// GetHandler returns the analytics handler
func (m *Module) GetHandler() *Handler {
	return m.handler
}

// DefaultConfig returns the default analytics configuration
func DefaultConfig() *Config {
	return &Config{
		PrometheusURL: "http://localhost:9090",
		Timeout:       30000000000, // 30 seconds in nanoseconds
		Namespace:     "stargate",
		Subsystem:     "gateway",
		DefaultRange:  "24h",
	}
}

// ValidateConfig validates the analytics configuration
func ValidateConfig(config *Config) error {
	if config == nil {
		return fmt.Errorf("analytics config cannot be nil")
	}

	if config.PrometheusURL == "" {
		return fmt.Errorf("prometheus_url is required")
	}

	if config.Namespace == "" {
		config.Namespace = "stargate"
	}

	if config.Subsystem == "" {
		config.Subsystem = "gateway"
	}

	if config.DefaultRange == "" {
		config.DefaultRange = "24h"
	}

	// Validate default range
	if _, err := ParseTimeRange(config.DefaultRange); err != nil {
		return fmt.Errorf("invalid default_range: %w", err)
	}

	return nil
}

// Builder helps build the analytics module with configuration
type Builder struct {
	config  *Config
	appRepo portal.ApplicationRepository
	logger  log.Logger
}

// NewBuilder creates a new analytics module builder
func NewBuilder() *Builder {
	return &Builder{
		config: DefaultConfig(),
	}
}

// WithConfig sets the analytics configuration
func (b *Builder) WithConfig(config *Config) *Builder {
	b.config = config
	return b
}

// WithPrometheusURL sets the Prometheus URL
func (b *Builder) WithPrometheusURL(url string) *Builder {
	if b.config == nil {
		b.config = DefaultConfig()
	}
	b.config.PrometheusURL = url
	return b
}

// WithNamespace sets the metrics namespace
func (b *Builder) WithNamespace(namespace string) *Builder {
	if b.config == nil {
		b.config = DefaultConfig()
	}
	b.config.Namespace = namespace
	return b
}

// WithSubsystem sets the metrics subsystem
func (b *Builder) WithSubsystem(subsystem string) *Builder {
	if b.config == nil {
		b.config = DefaultConfig()
	}
	b.config.Subsystem = subsystem
	return b
}

// WithApplicationRepository sets the application repository
func (b *Builder) WithApplicationRepository(repo portal.ApplicationRepository) *Builder {
	b.appRepo = repo
	return b
}

// WithLogger sets the logger
func (b *Builder) WithLogger(logger log.Logger) *Builder {
	b.logger = logger
	return b
}

// Build builds the analytics module
func (b *Builder) Build() (*Module, error) {
	if b.appRepo == nil {
		return nil, fmt.Errorf("application repository is required")
	}

	if b.logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	if err := ValidateConfig(b.config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return NewModule(b.config, b.appRepo, b.logger)
}

// Integration helpers for existing portal backend

// IntegrateWithPortal integrates analytics module with the existing portal backend
func IntegrateWithPortal(router *gin.RouterGroup, appRepo portal.ApplicationRepository, config *Config, logger log.Logger) (*Module, error) {
	// Build analytics module
	module, err := NewBuilder().
		WithConfig(config).
		WithApplicationRepository(appRepo).
		WithLogger(logger).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build analytics module: %w", err)
	}

	// Register routes under /api prefix
	apiGroup := router.Group("/api")
	module.RegisterRoutes(apiGroup)

	return module, nil
}

// ConfigFromEnv creates analytics config from environment variables
func ConfigFromEnv() *Config {
	// This would typically read from environment variables
	// For now, return default config
	return DefaultConfig()
}
